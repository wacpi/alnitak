package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

// getMP4InitRange 从 fMP4 文件中提取初始化范围
// 返回: initRange (0 到 moov 结束), indexRange (sidx 范围，如果没有 sidx 则用整个文件)
// 用于 DASH SegmentBase 模式
func getMP4InitRange(filePath string) (initRange, indexRange string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", "", err
	}
	fileSize := fileInfo.Size()

	// 遍历 MP4 box 结构，找到 ftyp, moov, sidx
	var moovEnd int64 = 0
	var sidxStart int64 = 0
	var sidxEnd int64 = 0

	offset := int64(0)

parseLoop:
	for offset < fileSize {
		boxSize, boxType, err := readMP4BoxSizeAndType(file, offset, fileSize)
		if err != nil || boxSize <= 0 {
			break
		}

		switch boxType {
		case "moov":
			moovEnd = offset + boxSize
		case "sidx":
			sidxStart = offset
			sidxEnd = offset + boxSize
		case "moof":
			// 遇到 moof 就停止，后面都是媒体数据
			break parseLoop
		}

		offset += boxSize

		// 如果找到了 sidx，可以停止了
		if sidxEnd > 0 {
			break
		}
	}

	if moovEnd == 0 {
		return "", "", fmt.Errorf("moov box not found in %s", filePath)
	}

	// initRange: 从文件开头到 moov 结束（ftyp + moov）
	initRange = fmt.Sprintf("0-%d", moovEnd-1)

	// indexRange: 优先使用 sidx，否则使用整个文件范围
	if sidxEnd > 0 {
		// 有 sidx box，使用 sidx 的范围
		indexRange = fmt.Sprintf("%d-%d", sidxStart, sidxEnd-1)
	} else {
		// 没有 sidx，使用整个文件作为索引范围（从 moov 结束后开始）
		// 对于 fMP4 音视频分离，播放器需要能够请求任意位置的数据
		indexRange = fmt.Sprintf("%d-%d", moovEnd, fileSize-1)
	}

	return initRange, indexRange, nil
}

type TranscodingTarget struct {
	Resolution  string // 分辨率
	BitrateRate string // 码率
	FPS         string // 帧率
	FpsName     string // 帧率名称
}

// 转码进程信息
type TranscodingProcess struct {
	VideoID    uint
	ResourceID uint
	PID        int
	Cmd        *exec.Cmd
	CancelFunc context.CancelFunc
	OutputDir  string
}

type resourceTranscodingProgress struct {
	VideoID    uint
	ResourceID uint
	Details    map[string]vo.TranscodingProgressItem // key: quality
}

// 全局转码并发控制
var (
	transcodingSemaphore chan struct{} // 信号量，控制同时转码的任务数
	semaphoreOnce        sync.Once
	gpuAvailable         = true                                // GPU是否可用
	gpuCheckMutex        sync.RWMutex                          // 保护GPU状态的读写锁
	gpuFailCount         = 0                                   // GPU连续失败次数
	maxGpuFailCount      = maxGpuFailCountThreshold             // 最大允许GPU失败次数
	transcodingProcesses = make(map[uint][]TranscodingProcess) // key: videoID, value: 该视频的所有转码进程
	processMapMutex      sync.RWMutex                          // 保护进程映射的读写锁
	transcodingProgress  = make(map[uint]*resourceTranscodingProgress)
	progressMutex        sync.RWMutex
)

// 初始化转码并发控制（根据CPU核心数或配置）
func initTranscodingSemaphore() {
	semaphoreOnce.Do(func() {
		var maxConcurrent int
		if global.Config.Transcoding.UseGpu {
			maxConcurrent = global.Config.Transcoding.MaxGpuConcurrency
			if maxConcurrent <= 0 {
				maxConcurrent = maxGPUConcurrentTranscoding
			}
		} else {
			maxConcurrent = global.Config.Transcoding.MaxCpuConcurrency
			if maxConcurrent <= 0 {
				maxConcurrent = maxCPUConcurrentTranscoding
			}
		}
		transcodingSemaphore = make(chan struct{}, maxConcurrent)
		utils.InfoLog(fmt.Sprintf("【转码并发控制初始化】最大并发数=%d (GPU=%v)", maxConcurrent, global.Config.Transcoding.UseGpu), "transcoding")
	})
}

// 检查GPU是否可用
func checkGPUAvailable() bool {
	gpuCheckMutex.RLock()
	defer gpuCheckMutex.RUnlock()
	return gpuAvailable
}

// GPU失败处理
func handleGPUFailure() {
	gpuCheckMutex.Lock()
	defer gpuCheckMutex.Unlock()

	gpuFailCount++
	utils.InfoLog(fmt.Sprintf("【GPU失败】失败次数=%d/%d", gpuFailCount, maxGpuFailCount), "transcoding")

	if gpuFailCount >= maxGpuFailCount {
		gpuAvailable = false
		utils.InfoLog("【GPU禁用】连续失败次数达到阈值，自动切换到CPU模式", "transcoding")
	}
}

// ResetGPUState 重置GPU状态，用于重新转码时恢复GPU
func ResetGPUState() {
	gpuCheckMutex.Lock()
	defer gpuCheckMutex.Unlock()

	if gpuFailCount > 0 || !gpuAvailable {
		oldFailCount := gpuFailCount
		oldAvailable := gpuAvailable
		gpuFailCount = 0
		gpuAvailable = true
		utils.InfoLog(fmt.Sprintf("【GPU重置】失败次数: %d→0, 可用状态: %v→true", oldFailCount, oldAvailable), "transcoding")
	}
}

func initResourceTranscodingProgress(videoID, resourceID uint, targets []TranscodingTarget) {
	progressMutex.Lock()
	defer progressMutex.Unlock()

	state := &resourceTranscodingProgress{
		VideoID:    videoID,
		ResourceID: resourceID,
		Details:    make(map[string]vo.TranscodingProgressItem, len(targets)),
	}
	for _, target := range targets {
		quality := target.Resolution + "_" + target.BitrateRate + "_" + target.FpsName
		state.Details[quality] = vo.TranscodingProgressItem{
			ResourceID: resourceID,
			Quality:    quality,
			Progress:   0,
			Status:     "waiting",
		}
	}
	transcodingProgress[resourceID] = state
}

func updateTranscodingQualityProgress(resourceID uint, quality string, progress float64, status string) {
	progressMutex.Lock()
	defer progressMutex.Unlock()

	state, ok := transcodingProgress[resourceID]
	if !ok {
		return
	}
	item, exists := state.Details[quality]
	if !exists {
		item = vo.TranscodingProgressItem{
			ResourceID: resourceID,
			Quality:    quality,
			Status:     "processing",
		}
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	item.Progress = progress
	if status != "" {
		item.Status = status
	}
	state.Details[quality] = item
}

func markTranscodingQualityFailed(resourceID uint, quality string) {
	updateTranscodingQualityProgress(resourceID, quality, 0, "fail")
}

func clearResourceTranscodingProgress(resourceID uint) {
	progressMutex.Lock()
	defer progressMutex.Unlock()
	delete(transcodingProgress, resourceID)
}

func GetVideoTranscodingProgress(videoID uint) (float64, []vo.TranscodingProgressItem) {
	progressMutex.RLock()
	defer progressMutex.RUnlock()

	details := make([]vo.TranscodingProgressItem, 0)
	totalProgress := 0.0
	totalCount := 0

	for _, state := range transcodingProgress {
		if state.VideoID != videoID {
			continue
		}
		for _, item := range state.Details {
			details = append(details, item)
			totalProgress += item.Progress
			totalCount++
		}
	}

	if totalCount == 0 {
		return 0, details
	}

	sort.Slice(details, func(i, j int) bool {
		iw, ih, ifps, ik := parseProgressQualitySortKey(details[i].Quality)
		jw, jh, jfps, jk := parseProgressQualitySortKey(details[j].Quality)

		if ih != jh {
			return ih > jh
		}
		if iw != jw {
			return iw > jw
		}
		if ifps != jfps {
			return ifps > jfps
		}
		if ik != jk {
			return ik > jk
		}
		return details[i].Quality > details[j].Quality
	})

	return totalProgress / float64(totalCount), details
}

func parseProgressQualitySortKey(quality string) (width, height, fps, bitrateK int) {
	// quality 形如: 1920x1080_8000k_60
	parts := strings.Split(quality, "_")
	if len(parts) >= 1 {
		res := strings.Split(parts[0], "x")
		if len(res) == 2 {
			width, _ = strconv.Atoi(res[0])
			height, _ = strconv.Atoi(res[1])
		}
	}
	if len(parts) >= 2 {
		bitrateK = parseBitrateKbps(parts[1])
	}
	if len(parts) >= 3 {
		fps, _ = strconv.Atoi(parts[2])
	}
	return
}

// 生成封面
func GenerateCover(inputFile, outputFile string) error {
	command := []string{"-i", inputFile, "-vframes", "1", "-y", outputFile}

	_, err := utils.RunCmd(exec.Command("ffmpeg", command...))
	if err != nil {
		utils.ErrorLog("提取封面失败", "transcoding", err.Error())
		return err
	}
	return nil
}

// 获取视频信息
func ProcessVideoInfo(input string) (*dto.TranscodingInfo, error) {
	transcodingInfo := dto.TranscodingInfo{OriginalVideoStatus: -1}
	videoData, err := getVideoInfo(input)
	if err != nil {
		utils.ErrorLog("读取视频信息失败", "transcoding", err.Error())
		return &transcodingInfo, err
	}

	// 从流中分别找到视频流和音频流
	var videoStream *global.Streams
	var audioStream *global.Streams
	for i := range videoData.Stream {
		switch videoData.Stream[i].CodecType {
		case "video":
			if videoStream == nil {
				videoStream = &videoData.Stream[i]
			}
		case "audio":
			if audioStream == nil {
				audioStream = &videoData.Stream[i]
			}
		}
	}

	// 兼容旧逻辑：如果按 codec_type 没找到视频流，回退到 Stream[0]
	if videoStream == nil && len(videoData.Stream) > 0 {
		videoStream = &videoData.Stream[0]
	}

	if videoStream == nil {
		return &transcodingInfo, fmt.Errorf("未找到视频流: %s", input)
	}

	// 计算最大分辨率
	transcodingInfo.Width = videoStream.Width
	transcodingInfo.Height = videoStream.Height
	transcodingInfo.CodecName = videoStream.CodecName

	// 获取视频时长（优先使用 stream.duration，若为空则使用 format.duration）
	durationStr := videoStream.Duration
	if durationStr == "" {
		durationStr = videoData.Format.Duration
	}
	transcodingInfo.Duration, _ = strconv.ParseFloat(durationStr, 64)

	// 获取帧率（使用 AvgFrameRate，转码时会统一转换为标准30帧）
	transcodingInfo.FPS = videoStream.AvgFrameRate
	transcodingInfo.FPS30, transcodingInfo.FPS60 = getFpsInfo(transcodingInfo.FPS)

	// 优先使用视频流码率（更准确）
	if br, err := strconv.Atoi(videoStream.BitRate); err == nil && br > 0 {
		transcodingInfo.VideoBitRate = br
	}

	// 提取音频流参数（动态探测，最大可能保持源文件音频特性）
	if audioStream != nil {
		// 采样率
		if sr, err := strconv.Atoi(audioStream.SampleRate); err == nil && sr > 0 {
			transcodingInfo.AudioSampleRate = sr
		}
		// 声道数
		if audioStream.Channels > 0 {
			transcodingInfo.AudioChannels = audioStream.Channels
		}
		// 码率（ffprobe 返回 bps 字符串，如 "320000"）
		if br, err := strconv.Atoi(audioStream.BitRate); err == nil && br > 0 {
			transcodingInfo.AudioBitRate = br
		}
		utils.InfoLog(fmt.Sprintf("【音频探测】采样率=%d, 声道数=%d, 码率=%d bps",
			transcodingInfo.AudioSampleRate, transcodingInfo.AudioChannels, transcodingInfo.AudioBitRate), "transcoding")
	}

	// 设置默认值（源文件缺失音频信息时的兜底）
	if transcodingInfo.AudioSampleRate == 0 {
		transcodingInfo.AudioSampleRate = 48000
	}
	if transcodingInfo.AudioChannels == 0 {
		transcodingInfo.AudioChannels = 2
	}
	if transcodingInfo.AudioBitRate == 0 {
		transcodingInfo.AudioBitRate = 320000
	}

	// 音频码率上限限制：最高 320kbps，避免不合理的高码率
	if transcodingInfo.AudioBitRate > 320000 {
		transcodingInfo.AudioBitRate = 320000
	}

	// 回退：当视频流未提供码率时，使用 format.bit_rate - audioBitRate 估算视频码率
	if transcodingInfo.VideoBitRate == 0 {
		if totalBitRate, err := strconv.Atoi(videoData.Format.BitRate); err == nil && totalBitRate > 0 {
			if totalBitRate > transcodingInfo.AudioBitRate {
				transcodingInfo.VideoBitRate = totalBitRate - transcodingInfo.AudioBitRate
			} else {
				transcodingInfo.VideoBitRate = totalBitRate
			}
		}
	}

	// 兜底：仍获取不到源码率时，按最高分辨率档默认码率
	if transcodingInfo.VideoBitRate == 0 {
		maxLevel := getMaxQualityLevel(transcodingInfo.Width, transcodingInfo.Height)
		transcodingInfo.VideoBitRate = getDefaultVideoBitRateByLevel(maxLevel, transcodingInfo.Width < transcodingInfo.Height)
	}

	utils.InfoLog(fmt.Sprintf("【视频探测】分辨率=%dx%d, 源视频码率=%d bps, 帧率=%s, 60fps可用=%v",
		transcodingInfo.Width, transcodingInfo.Height, transcodingInfo.VideoBitRate, transcodingInfo.FPS, transcodingInfo.FPS60 != ""), "transcoding")

	return &transcodingInfo, nil
}

func VideoTransCoding(transcodingInfo *dto.TranscodingInfo) {
	// 实例级音频编码状态（每次转码调用独立，避免多视频并发冲突）
	var audioEncoded bool
	var audioFailed bool
	var audioMu sync.Mutex
	audioDone := make(chan struct{}) // 音频编码完成信号（成功或失败都会关闭）
	var audioDoneOnce sync.Once

	// 初始化并发控制
	initTranscodingSemaphore()

	var wg sync.WaitGroup
	targets := getTranscodingTarget(transcodingInfo)
	initResourceTranscodingProgress(transcodingInfo.VideoID, transcodingInfo.ResourceID, targets)
	wg.Add(len(targets))

	utils.InfoLog(fmt.Sprintf("【转码开始】VideoID=%d, ResourceID=%d, 目标数量=%d",
		transcodingInfo.VideoID, transcodingInfo.ResourceID, len(targets)), "transcoding")

	successCount := 0
	var mu sync.Mutex // 保护successCount

	for _, v := range targets {
		c := v // 处理协程引用循环变量问题
		go func() {
			defer wg.Done()
			fileName := c.Resolution + "_" + c.BitrateRate + "_" + c.FpsName
			needEncodeAudio := false

			defer func() {
				if r := recover(); r != nil {
					markTranscodingQualityFailed(transcodingInfo.ResourceID, fileName)
					utils.ErrorLog(fmt.Sprintf("【Goroutine panic】%s: %v", fileName, r), "transcoding", "")
				}
				// 如果本 goroutine 负责音频编码，确保异常退出时（panic/cancel）通知所有等待者
				if needEncodeAudio {
					audioDoneOnce.Do(func() {
						audioMu.Lock()
						audioFailed = true
						audioMu.Unlock()
						close(audioDone)
						markAllTranscodingQualitiesFailed(transcodingInfo.ResourceID)
					})
				}
			}()

			// 获取转码资源锁（控制并发数）
			transcodingSemaphore <- struct{}{}
			defer func() { <-transcodingSemaphore }() // 释放资源锁

			// 拿到锁后，将状态从 waiting 更新为 processing
			updateTranscodingQualityProgress(transcodingInfo.ResourceID, fileName, 0, "processing")

			// 创建可取消的context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			videoFile := transcodingInfo.OutputDir + fileName + "_video.m4s"
			audioFile := transcodingInfo.OutputDir + "audio.m4s"

			utils.InfoLog(fmt.Sprintf("【开始SegmentBase转码】%s", fileName), "transcoding")

			// ========== 编码视频 ==========
			var videoErr error
			useGpu := global.Config.Transcoding.UseGpu && checkGPUAvailable()

			utils.InfoLog(fmt.Sprintf("【编码决策】%s 使用GPU=%v", fileName, useGpu), "transcoding")

			if useGpu {
				utils.InfoLog(fmt.Sprintf("【GPU编码视频】%s", fileName), "transcoding")
				videoErr = encodeVideoOnlyGPU(ctx, transcodingInfo.VideoID, transcodingInfo.ResourceID,
					transcodingInfo.InputFile, videoFile, c.Resolution, c.BitrateRate, c.FPS, fileName, transcodingInfo.Duration, cancel)

				if videoErr != nil && strings.Contains(videoErr.Error(), "GPU error") {
					handleGPUFailure()
					if ctx.Err() != nil {
						utils.InfoLog(fmt.Sprintf("【跳过CPU降级】%s，转码已取消", fileName), "transcoding")
						return
					}
					utils.InfoLog(fmt.Sprintf("【降级CPU编码视频】%s", fileName), "transcoding")
					videoErr = encodeVideoOnly(ctx, transcodingInfo.VideoID, transcodingInfo.ResourceID,
						transcodingInfo.InputFile, videoFile, c.Resolution, c.BitrateRate, c.FPS, fileName, transcodingInfo.Duration, cancel)
				} else if videoErr == nil {
					gpuCheckMutex.Lock()
					if gpuFailCount > 0 {
						gpuFailCount = 0
						utils.InfoLog("【GPU恢复】转码成功，重置失败计数", "transcoding")
					}
					gpuCheckMutex.Unlock()
				}
			} else {
				utils.InfoLog(fmt.Sprintf("【CPU编码视频】%s", fileName), "transcoding")
				videoErr = encodeVideoOnly(ctx, transcodingInfo.VideoID, transcodingInfo.ResourceID,
					transcodingInfo.InputFile, videoFile, c.Resolution, c.BitrateRate, c.FPS, fileName, transcodingInfo.Duration, cancel)
			}

			if videoErr != nil {
				markTranscodingQualityFailed(transcodingInfo.ResourceID, fileName)
				utils.ErrorLog(fmt.Sprintf("【视频编码失败】%s", fileName), "transcoding", videoErr.Error())
				return
			}

			// 检查context是否被取消（在视频编码后）
			if ctx.Err() != nil {
				utils.InfoLog(fmt.Sprintf("【转码取消】%s，context已取消", fileName), "transcoding")
				return
			}

			// ========== 编码音频（只编码一次）==========
			audioMu.Lock()
			needEncodeAudio = !audioEncoded
			if needEncodeAudio {
				audioEncoded = true
			}
			audioMu.Unlock()

			// 检查context是否被取消（音频编码前）
			if ctx.Err() != nil {
				utils.InfoLog(fmt.Sprintf("【转码取消】%s，context已取消", fileName), "transcoding")
				return
			}

			if needEncodeAudio {
				utils.InfoLog(fmt.Sprintf("【编码音频】audio.m4s (码率=%dk, 采样率=%d, 声道=%d)",
					transcodingInfo.AudioBitRate/1000, transcodingInfo.AudioSampleRate, transcodingInfo.AudioChannels), "transcoding")
				if err := encodeAudioOnly(ctx, transcodingInfo.InputFile, audioFile,
					transcodingInfo.AudioBitRate, transcodingInfo.AudioSampleRate, transcodingInfo.AudioChannels); err != nil {
					utils.ErrorLog("【音频编码失败】", "transcoding", err.Error())
					audioMu.Lock()
					audioFailed = true
					audioMu.Unlock()
					audioDoneOnce.Do(func() { close(audioDone) })
					markAllTranscodingQualitiesFailed(transcodingInfo.ResourceID)
					return
				}
				// 音频编码成功，通知所有等待者
				audioDoneOnce.Do(func() { close(audioDone) })
			}

			// 等待音频编码完成
			if !needEncodeAudio {
				select {
				case <-audioDone:
					audioMu.Lock()
					failed := audioFailed
					audioMu.Unlock()
					if failed {
						markTranscodingQualityFailed(transcodingInfo.ResourceID, fileName)
						utils.ErrorLog(fmt.Sprintf("【音频编码已失败】%s 放弃等待", fileName), "transcoding", "")
						return
					}
				case <-ctx.Done():
					utils.InfoLog(fmt.Sprintf("【转码取消】%s，等待音频时context已取消", fileName), "transcoding")
					return
				}
			}

			// ========== 保存到数据库 ==========
			width, height, bandwidth, frameRate := parseQualityInfo(fileName)

			// 从 fMP4 文件提取实际的 moov box 范围
			videoFilePath := transcodingInfo.OutputDir + fileName + "_video.m4s"
			audioFilePath := transcodingInfo.OutputDir + "audio.m4s"

			videoInitRange, videoIndexRange, err := getMP4InitRange(videoFilePath)
			if err != nil {
				markTranscodingQualityFailed(transcodingInfo.ResourceID, fileName)
				utils.ErrorLog(fmt.Sprintf("【视频InitRange提取失败】%s", fileName), "transcoding", err.Error())
				return
			}
			audioInitRange, audioIndexRange, err := getMP4InitRange(audioFilePath)
			if err != nil {
				markTranscodingQualityFailed(transcodingInfo.ResourceID, fileName)
				utils.ErrorLog(fmt.Sprintf("【音频InitRange提取失败】%s", fileName), "transcoding", err.Error())
				return
			}

			videoCodec := "avc1.640028" // 兜底：保证可播放性
			if c, err := probeH264Avc1CodecString(videoFilePath); err != nil {
				utils.ErrorLog("探测 H264 codec 失败，使用默认值兜底", "transcoding", err.Error())
			} else {
				videoCodec = c
			}

			utils.InfoLog(fmt.Sprintf("【范围提取】%s video init=%s, index=%s; audio init=%s, index=%s",
				fileName, videoInitRange, videoIndexRange, audioInitRange, audioIndexRange), "transcoding")

			indexFile := &model.VideoIndexFile{
				ResourceID:    transcodingInfo.ResourceID,
				Quality:       fileName,
				DirName:       transcodingInfo.DirName,
				TotalDuration: transcodingInfo.Duration,

				// 视频流信息
				VideoFile:       fileName + "_video.m4s",
				VideoBandwidth:  bandwidth,
				VideoCodec:      videoCodec,
				Width:           width,
				Height:          height,
				FrameRate:       frameRate,
				VideoInitRange:  videoInitRange,
				VideoIndexRange: videoIndexRange,

				// 音频流信息（使用源文件探测的动态参数）
				AudioFile:       "audio.m4s",
				AudioBandwidth:  transcodingInfo.AudioBitRate,
				AudioCodec:      "mp4a.40.2",
				AudioSampleRate: transcodingInfo.AudioSampleRate,
				AudioInitRange:  audioInitRange,
				AudioIndexRange: audioIndexRange,
			}

			if err := global.Mysql.Create(indexFile).Error; err != nil {
				markTranscodingQualityFailed(transcodingInfo.ResourceID, fileName)
				utils.ErrorLog(fmt.Sprintf("【保存索引失败】%s", fileName), "transcoding", err.Error())
				return
			}

			updateTranscodingQualityProgress(transcodingInfo.ResourceID, fileName, 100, "success")
			utils.InfoLog(fmt.Sprintf("【成功】%s 转码完成, video=%s, audio=%s",
				fileName, indexFile.VideoFile, indexFile.AudioFile), "transcoding")

			mu.Lock()
			successCount++
			mu.Unlock()
		}()
	}

	wg.Wait()

	utils.InfoLog(fmt.Sprintf("【所有转码任务完成】成功=%d, 总数=%d", successCount, len(targets)), "transcoding")

	// 【关键】检查是否所有任务都失败了（可能是被用户取消）
	// 如果成功数为0，说明转码被取消或全部失败，此时也必须更新稿件/资源状态为失败，不能一直停留在转码中
	if successCount == 0 {
		utils.InfoLog(fmt.Sprintf("【所有转码失败】VideoID=%d, ResourceID=%d, 调用completeTransCoding标记为PROCESSING_FAIL",
			transcodingInfo.VideoID, transcodingInfo.ResourceID), "transcoding")
		completeTransCoding(transcodingInfo.VideoID, transcodingInfo.ResourceID, global.PROCESSING_FAIL, transcodingInfo.OriginalVideoStatus)
		return
	}

	// 上传oss - 添加panic恢复
	defer func() {
		if r := recover(); r != nil {
			utils.ErrorLog("【OSS上传panic】", "transcoding", fmt.Sprintf("%v", r))
			utils.InfoLog("【调用completeTransCoding】status=PROCESSING_FAIL（OSS panic）", "transcoding")
			completeTransCoding(transcodingInfo.VideoID, transcodingInfo.ResourceID, global.PROCESSING_FAIL, transcodingInfo.OriginalVideoStatus)
		}
	}()

	utils.InfoLog(fmt.Sprintf("【开始后续处理】VideoID=%d, ResourceID=%d", transcodingInfo.VideoID, transcodingInfo.ResourceID), "transcoding")

	if global.Config.Storage.OssType != "local" {
		utils.InfoLog(fmt.Sprintf("【开始上传OSS】OssType=%s", global.Config.Storage.OssType), "transcoding")

		files, err := os.ReadDir(transcodingInfo.OutputDir)
		if err != nil {
			utils.ErrorLog("读取视频文件夹失败", "oss", err.Error())
			utils.InfoLog("【调用completeTransCoding】status=PROCESSING_FAIL（OSS失败）", "transcoding")
			completeTransCoding(transcodingInfo.VideoID, transcodingInfo.ResourceID, global.PROCESSING_FAIL, transcodingInfo.OriginalVideoStatus)
			return
		}

		// 并发上传文件
		uploadCount := uploadFilesToOSS(transcodingInfo.DirName, transcodingInfo.OutputDir, transcodingInfo.Suffix, files)
		utils.InfoLog(fmt.Sprintf("【OSS上传完成】成功上传=%d/%d个文件", uploadCount, len(files)), "transcoding")

		if uploadCount == 0 {
			utils.ErrorLog("【OSS上传全部失败】无文件上传成功", "transcoding", "")
			completeTransCoding(transcodingInfo.VideoID, transcodingInfo.ResourceID, global.PROCESSING_FAIL, transcodingInfo.OriginalVideoStatus)
			return
		}
	} else {
		utils.InfoLog("【跳过OSS上传】使用本地存储", "transcoding")
	}

	// 更新状态
	utils.InfoLog(fmt.Sprintf("【调用completeTransCoding】VideoID=%d, ResourceID=%d, status=WAITING_REVIEW",
		transcodingInfo.VideoID, transcodingInfo.ResourceID), "transcoding")
	completeTransCoding(transcodingInfo.VideoID, transcodingInfo.ResourceID, global.WAITING_REVIEW, transcodingInfo.OriginalVideoStatus)
}

// getMaxQualityLevel 根据源视频分辨率确定最高分辨率档位（YouTube风格，兼容接近1080p的非标准尺寸）
// 1080p/720p/480p/360p 指的是短边像素数，但对于长边>=1920且短边>=1000的情况，也视为1080级别
func getMaxQualityLevel(width, height int) int {
	shortSide := width
	longSide := height
	if height < width {
		shortSide = height
		longSide = width
	}

	// 兼容 1920x1038 等接近 1080p 的非标准分辨率：
	// 只要长边达到 1920 且短边不低于 1000，就视为 1080 档
	if longSide >= 1920 && shortSide >= 1000 {
		return 1080
	}

	if shortSide >= 1080 {
		return 1080
	}
	if shortSide >= 720 {
		return 720
	}
	if shortSide >= 480 {
		return 480
	}
	return 360
}

// 获取帧率信息
func getFpsInfo(avgFrameRate string) (string, string) {
	parts := strings.Split(avgFrameRate, "/")
	if len(parts) == 2 {
		numerator := utils.StringToInt(parts[0])
		denominator := utils.StringToInt(parts[1])
		if denominator == 0 {
			return "30000/1000", "" // 统一使用30fps，时间基15360，与YouTube对齐
		}

		// 计算帧率
		fps := float64(numerator) / float64(denominator)
		// 低于30fps的视频使用原始帧率（中间格式转换后不再是AV1/VP9，可以直接使用）
		// 高于60fps的限制为60fps或30fps
		if fps < 30 {
			return avgFrameRate, ""
		}
		if fps >= 59 {
			// 59.94fps (60000/1001) 及以上，开启60帧率模式则用60，否则用30
			if global.Config.Transcoding.Generate1080p60 {
				return "30000/1000", "60000/1001" // FPS30始终为30fps，FPS60使用标准59.94时间基
			}
			return "30000/1000", "" // 没开60fps时，30fps用30
		}
	}

	// 30-60fps之间，统一使用30帧，时间基15360，与YouTube对齐
	return "30000/1000", ""
}

// qualityPreset 定义每个分辨率档位的参数（YouTube风格）
type qualityPreset struct {
	LongSide   int    // 长边像素数
	ShortSide  int    // 短边像素数
	BitrateH   string // 横屏码率
	BitrateV   string // 竖屏码率（像素少，码率适当降低）
	Bitrate60H string // 横屏60fps码率
	Bitrate60V string // 竖屏60fps码率
}

var qualityPresets = []qualityPreset{
	{1920, 1080, "8000k", "5000k", "12000k", "8000k"},
	{1280, 720, "5000k", "3000k", "7500k", "5000k"},
	{854, 480, "2500k", "1500k", "", ""},
	{640, 360, "1000k", "700k", "", ""},
}

// calcResolution 根据源视频宽高比和目标短边计算输出分辨率（保持宽高比，偶数对齐）
func calcResolution(srcWidth, srcHeight, targetShortSide int) (w, h int) {
	isPortrait := srcWidth < srcHeight
	srcAspect := float64(srcWidth) / float64(srcHeight)

	if isPortrait {
		// 竖屏：宽是短边
		w = targetShortSide
		h = int(math.Round(float64(w) / srcAspect))
	} else {
		// 横屏/正方形：高是短边
		h = targetShortSide
		w = int(math.Round(float64(h) * srcAspect))
	}
	// FFmpeg 要求偶数尺寸
	w = w &^ 1
	h = h &^ 1
	return
}

func parseBitrateKbps(rate string) int {
	rate = strings.TrimSpace(strings.TrimSuffix(rate, "k"))
	if rate == "" {
		return 0
	}
	v, err := strconv.Atoi(rate)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

func formatBitrateKbps(rateKbps int) string {
	if rateKbps < 1 {
		rateKbps = 1
	}
	return fmt.Sprintf("%dk", rateKbps)
}

func getDefaultVideoBitRateByLevel(maxLevel int, isPortrait bool) int {
	for _, preset := range qualityPresets {
		if preset.ShortSide != maxLevel {
			continue
		}
		bitrate := preset.BitrateH
		if isPortrait {
			bitrate = preset.BitrateV
		}
		if kbps := parseBitrateKbps(bitrate); kbps > 0 {
			return kbps * 1000
		}
	}
	// 理论不会走到这里，兜底给最低档横屏码率
	return 1000000
}

func scaleBitrateBySource(sourceMaxKbps, maxPresetKbps, currentPresetKbps int) int {
	if sourceMaxKbps <= 0 {
		return currentPresetKbps
	}
	if maxPresetKbps <= 0 || currentPresetKbps <= 0 {
		return sourceMaxKbps
	}
	if currentPresetKbps >= maxPresetKbps {
		return sourceMaxKbps
	}

	scaled := int(math.Round(float64(sourceMaxKbps) * float64(currentPresetKbps) / float64(maxPresetKbps)))
	if scaled < 200 {
		scaled = 200
	}
	if scaled > sourceMaxKbps {
		scaled = sourceMaxKbps
	}
	return scaled
}

func getPresetBitrateKbps(preset qualityPreset, isPortrait bool, fps60 bool) int {
	var bitrate string
	if fps60 {
		if isPortrait {
			bitrate = preset.Bitrate60V
		} else {
			bitrate = preset.Bitrate60H
		}
		if kbps := parseBitrateKbps(bitrate); kbps > 0 {
			return kbps
		}
	}

	if isPortrait {
		bitrate = preset.BitrateV
	} else {
		bitrate = preset.BitrateH
	}
	return parseBitrateKbps(bitrate)
}

// 获取转码目标（YouTube风格：根据源视频宽高比动态计算输出分辨率）
func getTranscodingTarget(videoInfo *dto.TranscodingInfo) []TranscodingTarget {
	targets := make([]TranscodingTarget, 0)
	maxLevel := getMaxQualityLevel(videoInfo.Width, videoInfo.Height)
	isPortrait := videoInfo.Width < videoInfo.Height

	var maxPreset qualityPreset
	for _, preset := range qualityPresets {
		if preset.ShortSide == maxLevel {
			maxPreset = preset
			break
		}
	}

	if maxPreset.ShortSide == 0 {
		return targets
	}

	maxPresetKbps30 := getPresetBitrateKbps(maxPreset, isPortrait, false)
	maxPresetKbps60 := getPresetBitrateKbps(maxPreset, isPortrait, true)
	if maxPresetKbps60 <= 0 {
		maxPresetKbps60 = maxPresetKbps30
	}
	sourceMaxKbps := videoInfo.VideoBitRate / 1000
	if sourceMaxKbps <= 0 {
		sourceMaxKbps = maxPresetKbps30
	}

	enable60 := global.Config.Transcoding.Generate1080p60 && videoInfo.FPS60 != ""

	for _, preset := range qualityPresets {
		if preset.ShortSide > maxLevel {
			continue
		}

		w, h := calcResolution(videoInfo.Width, videoInfo.Height, preset.ShortSide)
		resolution := fmt.Sprintf("%dx%d", w, h)

		currentPresetKbps30 := getPresetBitrateKbps(preset, isPortrait, false)
		dynamicKbps30 := scaleBitrateBySource(sourceMaxKbps, maxPresetKbps30, currentPresetKbps30)
		dynamicBitrate30 := formatBitrateKbps(dynamicKbps30)

		// 60fps 档位：仅生成源文件最高档，并且源帧率满足60fps
		if preset.ShortSide == maxLevel && enable60 {
			currentPresetKbps60 := getPresetBitrateKbps(preset, isPortrait, true)
			dynamicKbps60 := scaleBitrateBySource(sourceMaxKbps, maxPresetKbps60, currentPresetKbps60)
			dynamicBitrate60 := formatBitrateKbps(dynamicKbps60)
			targets = append(targets, TranscodingTarget{Resolution: resolution, BitrateRate: dynamicBitrate60, FPS: videoInfo.FPS60, FpsName: "60"})
		}

		targets = append(targets, TranscodingTarget{Resolution: resolution, BitrateRate: dynamicBitrate30, FPS: videoInfo.FPS30, FpsName: "30"})
	}

	return targets
}

// 获取视频信息
func getVideoInfo(input string) (info global.VideoInfo, err error) {
	cmd := exec.Command("ffprobe", "-i", input, "-v", "quiet", "-print_format", "json",
		"-show_format", "-show_streams",
		"-probesize", "5000000", "-analyzeduration", "5000000")
	out, err := utils.RunCmd(cmd)
	if err != nil {
		return info, err
	}

	// 反序列化
	err = json.Unmarshal(out.Bytes(), &info)
	if err != nil {
		return info, err
	}

	return info, nil
}

// parseFPS 解析帧率字符串，支持 "24000/1001" 或 "30" 格式
func parseFPS(fps string) float64 {
	if strings.Contains(fps, "/") {
		parts := strings.Split(fps, "/")
		if len(parts) == 2 {
			num, _ := strconv.ParseFloat(parts[0], 64)
			den, _ := strconv.ParseFloat(parts[1], 64)
			if den > 0 {
				return num / den
			}
		}
	}
	f, _ := strconv.ParseFloat(fps, 64)
	return f
}

func parseFfmpegClockToSeconds(clock string) float64 {
	parts := strings.Split(clock, ":")
	if len(parts) != 3 {
		return 0
	}
	hours, err1 := strconv.ParseFloat(parts[0], 64)
	mins, err2 := strconv.ParseFloat(parts[1], 64)
	secs, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0
	}
	return hours*3600 + mins*60 + secs
}

// ProbeH264Avc1CodecString 对本地可读的视频文件（如 *_video.m4s）执行 ffprobe，生成与转码落库一致的 avc1 字符串。
// 供维护脚本（如 cmd/fix_video_codec）使用。
func ProbeH264Avc1CodecString(filePath string) (string, error) {
	return probeH264Avc1CodecString(filePath)
}

// probeH264Avc1CodecString 生成 H.264 的 ISO BMFF codec 字符串形如 avc1.PPCCLL。
// 这里基于 ffprobe 的 profile/level 组合生成：
// - PP：profile_idc（baseline/main/high）
// - CC：profile_compatibility（你的转码显式使用 -profile:v high，实践中可按 00 处理）
// - LL：level_idc（ffprobe 的 level 例如 4.2 -> 42 -> 0x2A）
func probeH264Avc1CodecString(filePath string) (string, error) {
	// ffprobe 输出顺序与 show_entries 一致：profile、level
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=profile,level",
		"-of", "default=nw=1:nk=1",
		filePath,
	)

	out, err := utils.RunCmd(cmd)
	if err != nil {
		return "", err
	}

	raw := strings.TrimSpace(out.String())
	if raw == "" {
		return "", fmt.Errorf("ffprobe output empty")
	}

	lines := make([]string, 0, 2)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) < 2 {
		return "", fmt.Errorf("ffprobe output unexpected: %q", raw)
	}

	profile := strings.ToLower(lines[0])
	levelStr := strings.TrimSpace(lines[1])
	level, err := strconv.Atoi(levelStr)
	if err != nil {
		return "", fmt.Errorf("parse level failed: %w", err)
	}

	return avc1CodecStringFromH264ProfileLevel(profile, level)
}

// avc1CodecStringFromH264ProfileLevel 是纯函数：把 profile/level 映射到 avc1.PPCCLL。
// 注意：这里固定 CC=00，适配你当前转码链路（-profile:v high）。
func avc1CodecStringFromH264ProfileLevel(profile string, level int) (string, error) {
	profile = strings.ToLower(profile)
	var profileHex string
	switch {
	case strings.Contains(profile, "baseline"):
		profileHex = "42"
	case strings.Contains(profile, "main"):
		profileHex = "4D"
	case strings.Contains(profile, "high"):
		profileHex = "64"
	default:
		return "", fmt.Errorf("unsupported h264 profile: %s", profile)
	}

	levelHex := fmt.Sprintf("%02X", level)
	return fmt.Sprintf("avc1.%s00%s", profileHex, levelHex), nil
}

func watchFFmpegProgress(scanner *bufio.Scanner, resourceID uint, quality string, totalDuration float64) {
	if totalDuration <= 0 {
		return
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "out_time_ms=") {
			raw := strings.TrimPrefix(line, "out_time_ms=")
			ms, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				continue
			}
			seconds := ms / 1000000.0
			updateTranscodingQualityProgress(resourceID, quality, seconds/totalDuration*100, "processing")
			continue
		}
		if strings.HasPrefix(line, "out_time=") {
			raw := strings.TrimPrefix(line, "out_time=")
			seconds := parseFfmpegClockToSeconds(raw)
			updateTranscodingQualityProgress(resourceID, quality, seconds/totalDuration*100, "processing")
		}
	}
}

func markAllTranscodingQualitiesFailed(resourceID uint) {
	progressMutex.Lock()
	defer progressMutex.Unlock()

	state, ok := transcodingProgress[resourceID]
	if !ok {
		return
	}
	for quality, item := range state.Details {
		item.Progress = 0
		item.Status = "fail"
		state.Details[quality] = item
	}
}

func buildRateControlParams(rate string) (targetRate, maxRate, bufSize string) {
	targetRate = rate
	maxRate = rate
	bufSize = "4000k"
	if kbps, err := strconv.Atoi(strings.TrimSuffix(rate, "k")); err == nil && kbps > 0 {
		const peakRateFactor = 1.4
		maxKbps := int(math.Round(float64(kbps) * peakRateFactor))
		if maxKbps < kbps {
			maxKbps = kbps
		}
		maxRate = fmt.Sprintf("%dk", maxKbps)
		bufSize = fmt.Sprintf("%dk", maxKbps*2)
	}
	return
}

// SegmentBase模式：编码视频（CPU）
// 生成 fragmented MP4 (fMP4) 格式，包含 mvex box，用于 DASH SegmentBase
func encodeVideoOnly(ctx context.Context, videoID, resourceID uint, inputFile, outputFile, quality, rate, fps, progressQuality string, totalDuration float64, cancelFunc context.CancelFunc) error {
	// 解析帧率，支持 "24000/1001" 或 "30" 格式
	fpsFloat := parseFPS(fps)

	// YouTube/B站基本都是2秒GOP
	gopSize := int(math.Round(fpsFloat * 2))
	if gopSize < 1 {
		gopSize = 60
	}
	gopSizeStr := strconv.Itoa(gopSize)
	targetRate, maxrate, bufsize := buildRateControlParams(rate)

	// 分辨率缩放
	scaleFilter := fmt.Sprintf("scale=%s:flags=lanczos", quality)

	command := []string{
		"-i", inputFile,
		"-filter_complex", fmt.Sprintf("[0:v]setpts=PTS-STARTPTS,%s", scaleFilter),
		"-an",
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "20",
		"-tune", "film",
		"-profile:v", "high",
		"-pix_fmt", "yuv420p",
		"-bf", "3", // B 帧提升压缩；对齐由编码后 remuxVideoM4sPTSAlign 保证
		"-b_strategy", "2", // 自适应B帧决策
		"-flags", "+cgop", // 封闭GOP，确保P帧不跨GOP引用
		"-b:v", targetRate,
		"-maxrate", maxrate,
		"-bufsize", bufsize,
		"-r", fps,
		"-g", gopSizeStr,
		"-keyint_min", gopSizeStr,
		"-sc_threshold", "0",
		"-vsync", "cfr",
		"-progress", "pipe:1",
		"-nostats",
		"-f", "mp4",
		"-frag_duration", "2000000", // 每2秒一个fragment（微秒），与GOP对齐
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof+dash+global_sidx+negative_cts_offsets",
		"-avoid_negative_ts", "make_zero", // 时间戳从0开始，与YouTube对齐
		"-y", outputFile,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", command...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("视频编码启动失败: %s", err.Error())
	}

	// 打印完整命令
	utils.InfoLog(fmt.Sprintf("【CPU编码命令】ffmpeg %s", strings.Join(command, " ")), "transcoding")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("视频编码启动失败: %s, stderr: %s", err.Error(), stderr.String())
	}
	go watchFFmpegProgress(bufio.NewScanner(stdout), resourceID, progressQuality, totalDuration)

	registerTranscodingProcess(videoID, resourceID, cmd, cancelFunc, filepath.Dir(outputFile))
	defer unregisterTranscodingProcess(videoID, cmd.Process.Pid)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("视频编码失败: %s, stderr: %s", err.Error(), stderr.String())
	}

	return nil
}

// SegmentBase模式：编码视频（GPU）
// 生成 fragmented MP4 (fMP4) 格式，包含 mvex box，用于 DASH SegmentBase
func encodeVideoOnlyGPU(ctx context.Context, videoID, resourceID uint, inputFile, outputFile, quality, rate, fps, progressQuality string, totalDuration float64, cancelFunc context.CancelFunc) error {
	// 解析帧率，支持 "24000/1001" 或 "30" 格式
	fpsFloat := parseFPS(fps)
	gopSize := int(math.Round(fpsFloat * 2)) // 每2秒一个关键帧，与CPU编码和fragment对齐
	if gopSize < 1 {
		gopSize = 60 // 默认值（30fps * 2s）
	}
	gopSizeStr := strconv.Itoa(gopSize)
	targetRate, maxrate, bufsize := buildRateControlParams(rate)
	scaleFilter := fmt.Sprintf("scale=%s:flags=lanczos", quality)

	command := []string{
		"-i", inputFile,
		"-filter_complex", fmt.Sprintf("[0:v]setpts=PTS-STARTPTS,%s", scaleFilter),
		"-an",
		"-c:v", "h264_nvenc",
		"-cq", "20",
		"-preset", "p6",
		"-rc", "vbr",
		"-profile:v", "high",
		"-pix_fmt", "yuv420p",
		"-bf", "3", // B帧提升压缩效率
		"-b_ref_mode", "middle", // B帧参考模式，提升B帧质量
		"-rc-lookahead", "32", // 前瞻分析，改善码率分配
		"-temporal-aq", "1", // 时域自适应量化，提升运动场景质量
		"-spatial-aq", "1", // 空域自适应量化，提升细节保留
		"-aq-strength", "8", // AQ强度 (1-15，默认8)
		"-no-scenecut", "1", // 禁用场景检测插入额外关键帧
		"-forced-idr", "1", // 强制所有关键帧为IDR帧
		"-b:v", targetRate,
		"-maxrate", maxrate,
		"-bufsize", bufsize,
		"-r", fps,
		"-g", gopSizeStr, // 每2秒一个关键帧，与CPU编码和frag_duration对齐
		"-keyint_min", gopSizeStr, // 最小关键帧间距=GOP，防止nvenc插入额外关键帧
		"-strict_gop", "1",
		"-vsync", "cfr",
		"-progress", "pipe:1",
		"-nostats",
		"-f", "mp4",
		"-frag_duration", "2000000", // 每2秒一个fragment（微秒），与GOP对齐
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof+dash+global_sidx+negative_cts_offsets",
		"-avoid_negative_ts", "make_zero", // 时间戳从0开始，与YouTube对齐
		"-y", outputFile,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", command...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("GPU视频编码启动失败: %s", err.Error())
	}

	// 打印完整命令
	utils.InfoLog(fmt.Sprintf("【GPU编码命令】ffmpeg %s", strings.Join(command, " ")), "transcoding")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("GPU视频编码启动失败: %s, stderr: %s", err.Error(), stderr.String())
	}
	go watchFFmpegProgress(bufio.NewScanner(stdout), resourceID, progressQuality, totalDuration)

	registerTranscodingProcess(videoID, resourceID, cmd, cancelFunc, filepath.Dir(outputFile))
	defer unregisterTranscodingProcess(videoID, cmd.Process.Pid)

	if err := cmd.Wait(); err != nil {
		errOutput := stderr.String()
		if strings.Contains(errOutput, "No NVENC capable devices found") ||
			strings.Contains(errOutput, "Cannot load nvcuda.dll") ||
			strings.Contains(errOutput, "CUDA driver version is insufficient") ||
			strings.Contains(errOutput, "h264_nvenc") ||
			strings.Contains(errOutput, "nvenc") {
			return fmt.Errorf("GPU error: %s", errOutput)
		}
		return fmt.Errorf("GPU视频编码失败: %s, stderr: %s", err.Error(), errOutput)
	}

	return nil
}

// SegmentBase模式：编码音频
// 生成 fragmented MP4 (fMP4) 格式，包含 mvex box，用于 DASH SegmentBase
// 注意：音频没有关键帧概念，frag_keyframe 对纯音频无效，
// 必须用 -frag_duration 强制按时间分片，否则整个音频只有一个 fragment，
// 导致 HLS v7 byte-range 模式下 iOS 播放器无法正常加载
func encodeAudioOnly(ctx context.Context, inputFile, outputFile string, audioBitRate, audioSampleRate, audioChannels int) error {
	// 构建动态音频参数
	bitRateStr := fmt.Sprintf("%dk", audioBitRate/1000)
	sampleRateStr := strconv.Itoa(audioSampleRate)
	channelsStr := strconv.Itoa(audioChannels)

	command := []string{
		"-i", inputFile,
		"-filter_complex", "[0:a]asetpts=PTS-STARTPTS,aresample=async=1[aout]",
		"-map", "[aout]",
		"-vn",
		"-c:a", "aac",
		"-b:a", bitRateStr,
		"-ar", sampleRateStr,
		"-ac", channelsStr,
		"-f", "mp4",
		"-frag_duration", "2000000",
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof+dash+global_sidx",
		"-y", outputFile,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", command...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	// 打印音频编码命令
	utils.InfoLog(fmt.Sprintf("【音频编码命令】ffmpeg %s", strings.Join(command, " ")), "transcoding")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("音频编码失败: %s, stderr: %s", err.Error(), stderr.String())
	}

	return nil
}

// parseQualityInfo 从 quality 字符串解析分辨率、码率、帧率
func parseQualityInfo(quality string) (width, height, bandwidth int, frameRate float64) {
	width, height = 1920, 1080
	bandwidth = 3000000
	frameRate = 30

	parts := strings.Split(quality, "_")
	if len(parts) >= 1 {
		resParts := strings.Split(parts[0], "x")
		if len(resParts) == 2 {
			if w, err := strconv.Atoi(resParts[0]); err == nil {
				width = w
			}
			if h, err := strconv.Atoi(resParts[1]); err == nil {
				height = h
			}
		}
	}

	if len(parts) >= 2 {
		rateStr := strings.TrimSuffix(parts[1], "k")
		if rate, err := strconv.Atoi(rateStr); err == nil {
			bandwidth = rate * 1000
		}
	}

	if len(parts) >= 3 {
		if fr, err := strconv.ParseFloat(parts[2], 64); err == nil {
			frameRate = fr
		}
	}

	return
}

// 并发上传文件到OSS
func uploadFilesToOSS(dirName, outputDir, suffix string, files []os.DirEntry) int {
	maxConcurrency := ossUploadMaxConcurrency // 最大并发数
	utils.InfoLog(fmt.Sprintf("【OSS准备上传】文件总数=%d, 并发数=%d", len(files), maxConcurrency), "transcoding")

	// 创建任务通道和结果通道
	type uploadTask struct {
		index int
		file  os.DirEntry
	}

	tasks := make(chan uploadTask, len(files))
	results := make(chan bool, len(files))

	// 启动worker池
	var wg sync.WaitGroup
	for i := range maxConcurrency {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for task := range tasks {
				fileName := task.file.Name()

				// 跳过原始上传文件如果配置不上传
				originalFileName := "upload" + suffix
				if fileName == originalFileName && !global.Config.Storage.UploadMp4File {
					utils.InfoLog(fmt.Sprintf("【OSS跳过】%s (配置不上传原文件)", fileName), "transcoding")
					results <- false
					continue
				}

				objectKey := "video/" + dirName + "/" + fileName
				filePath := outputDir + fileName

				utils.InfoLog(fmt.Sprintf("【OSS上传中】Worker%d: %d/%d %s/%s", workerID, task.index+1, len(files), dirName, fileName), "transcoding")

				// 上传文件,失败重试1次
				err := global.Storage.PutObjectFromFile(objectKey, filePath)
				if err != nil {
					utils.ErrorLog(fmt.Sprintf("【OSS上传失败】%s,重试中...", fileName), "oss", err.Error())
					// 重试一次
					time.Sleep(ossUploadRetryDelay)
					err = global.Storage.PutObjectFromFile(objectKey, filePath)
				}

				if err != nil {
					utils.ErrorLog(fmt.Sprintf("【OSS上传失败】%s (重试后仍失败)", fileName), "oss", err.Error())
					results <- false
				} else {
					results <- true
				}
			}
		}(i)
	}

	// 发送任务
	for i, file := range files {
		if !file.IsDir() { // 只上传文件,不上传目录
			tasks <- uploadTask{index: i, file: file}
		}
	}
	close(tasks)

	// 等待所有worker完成
	wg.Wait()
	close(results)

	// 统计成功数量
	successCount := 0
	for success := range results {
		if success {
			successCount++
		}
	}

	return successCount
}

// 完成转码
func completeTransCoding(videoId, resourceId uint, status int, originalVideoStatus int) error {
	defer clearResourceTranscodingProgress(resourceId)
	utils.InfoLog("========== completeTransCoding 开始 ==========", "transcoding")
	utils.InfoLog(fmt.Sprintf("【入参】VideoID=%d, ResourceID=%d, 期望Status=%d", videoId, resourceId, status), "transcoding")

	// 查询是否存在转码成功的视频文件
	var videoFileCount int64
	global.Mysql.Model(&model.VideoIndexFile{}).Where("resource_id = ?", resourceId).Count(&videoFileCount)
	utils.InfoLog(fmt.Sprintf("【数据库查询】video_index_file表中resource_id=%d的记录数=%d", resourceId, videoFileCount), "transcoding")

	if videoFileCount == 0 {
		status = global.PROCESSING_FAIL
		utils.InfoLog("【状态修改】未生成任何视频文件，status改为PROCESSING_FAIL(3000)", "transcoding")
	}

	utils.InfoLog(fmt.Sprintf("【开始事务】准备更新status=%d", status), "transcoding")

	tx := global.Mysql.Begin()

	// 查询当前视频状态（加行锁，避免多资源同时完成时的竞态）
	var currentVideo model.Video
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&model.Video{}).Where("id = ?", videoId).First(&currentVideo).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog(fmt.Sprintf("【事务失败】查询视频失败或视频已删除，VideoID=%d", videoId), "transcoding", err.Error())
		return err
	}
	utils.InfoLog(fmt.Sprintf("【事务查询】VideoID=%d 当前status=%d", videoId, currentVideo.Status), "transcoding")

	// 查询当前资源状态
	var currentResource model.Resource
	if err := tx.Where("id = ?", resourceId).First(&currentResource).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog(fmt.Sprintf("【事务失败】查询资源失败或资源已删除，ResourceID=%d", resourceId), "transcoding", err.Error())
		return err
	}
	utils.InfoLog(fmt.Sprintf("【事务查询】ResourceID=%d 当前status=%d", resourceId, currentResource.Status), "transcoding")

	// 先检查是否所有资源都转码完成（包括当前这个资源）
	var processingCount int64
	tx.Model(&model.Resource{}).Where("vid = ? and status = ? and id != ?", videoId, global.VIDEO_PROCESSING, resourceId).Count(&processingCount)
	utils.InfoLog(fmt.Sprintf("【事务查询】VideoID=%d 除当前资源外,仍在转码中(status=200)的资源数=%d", videoId, processingCount), "transcoding")

	// 仅“重新转码”场景允许资源自动恢复为审核通过。
	// 普通新增分P（originalVideoStatus=-1）必须保持 WAITING_REVIEW，避免绕过人工审核。
	if status == global.WAITING_REVIEW && originalVideoStatus >= 0 && originalVideoStatus == global.AUDIT_APPROVED {
		status = global.AUDIT_APPROVED
		utils.InfoLog("【状态修正】重新转码场景恢复资源为AUDIT_APPROVED(0)", "transcoding")
	}

	// 先更新当前资源的状态（无论是否还有其他资源在转码）
	result := tx.Model(&model.Resource{}).Where("id = ? and status = ?", resourceId, global.VIDEO_PROCESSING).Updates(
		map[string]any{
			"status": status,
		},
	)
	if result.Error != nil {
		tx.Rollback()
		utils.ErrorLog("【事务失败】更新当前资源状态失败", "transcoding", result.Error.Error())
		return result.Error
	}
	utils.InfoLog(fmt.Sprintf("【事务执行】更新resource表 ResourceID=%d status=%d, 影响行数=%d", resourceId, status, result.RowsAffected), "transcoding")

	// 仅在资源转码成功时更新 VideoFile 为已就绪；失败场景不能误标记为 ready
	if status != global.PROCESSING_FAIL {
		updateVideoFileStatus(tx, currentResource, resourceId)
	} else {
		utils.InfoLog(fmt.Sprintf("【跳过VideoFile置ready】ResourceID=%d status=PROCESSING_FAIL", resourceId), "transcoding")
	}

	// 如果还有其他资源在转码中,只更新当前资源,不更新视频状态
	if processingCount > 0 {
		utils.InfoLog(fmt.Sprintf("【部分完成】还有%d个其他资源在转码中,仅更新当前资源状态,不更新视频状态", processingCount), "transcoding")
		if err := tx.Commit().Error; err != nil {
			utils.ErrorLog("【事务失败】提交事务失败", "transcoding", err.Error())
			return err
		}
		utils.InfoLog("【事务提交】成功（仅更新当前资源状态）", "transcoding")
		cache.DelVideoInfo(videoId)
		utils.InfoLog(fmt.Sprintf("【缓存清理】删除VideoID=%d的缓存", videoId), "transcoding")
		utils.InfoLog("========== completeTransCoding 结束 ==========", "transcoding")
		return nil
	}

	// 所有资源都转码完成了，准备更新视频状态
	utils.InfoLog("【判断】所有资源转码已完成,准备更新视频状态", "transcoding")

	// 检查所有资源是否都失败了
	var totalCount int64
	var failedCount int64
	tx.Model(&model.Resource{}).Where("vid = ?", videoId).Count(&totalCount)
	tx.Model(&model.Resource{}).Where("vid = ? and status = ?", videoId, global.PROCESSING_FAIL).Count(&failedCount)
	utils.InfoLog(fmt.Sprintf("【事务查询】VideoID=%d 总资源数=%d, 失败资源数=%d", videoId, totalCount, failedCount), "transcoding")

	var videoStatus int
	if failedCount == totalCount {
		// 所有资源都失败，视频状态设为处理失败
		videoStatus = global.PROCESSING_FAIL
		utils.InfoLog("【判断】全部资源失败，video status设为PROCESSING_FAIL(3000)", "transcoding")
	} else if currentVideo.Status == global.AUDIT_APPROVED {
		// 视频已审核通过（添加新分P场景），保持审核通过状态不变
		// 新分P由资源级别的状态独立控制审核，不影响已上线的视频
		videoStatus = global.AUDIT_APPROVED
		utils.InfoLog("【判断】视频已审核通过，保持AUDIT_APPROVED(0)不变（新分P独立审核）", "transcoding")
	} else if originalVideoStatus >= 0 && originalVideoStatus == global.AUDIT_APPROVED {
		// 重新转码且原始状态是审核通过，恢复为审核通过
		videoStatus = global.AUDIT_APPROVED
		utils.InfoLog("【判断】重新转码完成，恢复原始状态AUDIT_APPROVED(0)", "transcoding")
	} else {
		// 普通上传转码或原始状态非审核通过，设为待审核
		videoStatus = global.WAITING_REVIEW
		utils.InfoLog("【判断】至少一个资源成功，video status设为WAITING_REVIEW(500)", "transcoding")
	}

	// 更新视频状态
	videoResult := tx.Model(&model.Video{}).Where("id = ?", videoId).Updates(
		map[string]any{
			"status": videoStatus,
		},
	)
	if videoResult.Error != nil {
		tx.Rollback()
		utils.ErrorLog("【事务失败】更新视频状态失败", "transcoding", videoResult.Error.Error())
		return videoResult.Error
	}
	utils.InfoLog(fmt.Sprintf("【事务执行】更新video表 VideoID=%d status=%d, 影响行数=%d",
		videoId, videoStatus, videoResult.RowsAffected), "transcoding")

	if videoResult.RowsAffected == 0 {
		utils.InfoLog(fmt.Sprintf("【警告】video表更新影响0行！可能video.status已经是0或2000，当前status=%d", currentVideo.Status), "transcoding")
	}

	if err := tx.Commit().Error; err != nil {
		utils.ErrorLog("【事务失败】提交事务失败", "transcoding", err.Error())
		return err
	}

	utils.InfoLog("【事务提交】成功", "transcoding")

	// 转码完成后删除视频缓存，让下次查询时重新加载最新状态
	cache.DelVideoInfo(videoId)
	utils.InfoLog(fmt.Sprintf("【缓存清理】删除VideoID=%d的缓存", videoId), "transcoding")

	utils.InfoLog("========== completeTransCoding 结束 ==========", "transcoding")

	return nil
}

// updateVideoFileStatus 更新关联的 VideoFile 状态为已就绪（支持全局去重秒传）
// 每个资源转码完成后都应调用，而不是只在最后一个资源完成时调用
func updateVideoFileStatus(db *gorm.DB, currentResource model.Resource, resourceId uint) {
	if currentResource.FileID != 0 {
		result := db.Model(&model.VideoFile{}).Where("id = ? AND status != ?", currentResource.FileID, model.FileStatusReady).
			Update("status", model.FileStatusReady)
		if result.Error != nil {
			utils.ErrorLog(fmt.Sprintf("【警告】更新VideoFile状态失败, FileID=%d", currentResource.FileID), "transcoding", result.Error.Error())
		} else {
			utils.InfoLog(fmt.Sprintf("【VideoFile状态更新】FileID=%d 状态设为 FileStatusReady(4), 影响行数=%d", currentResource.FileID, result.RowsAffected), "transcoding")
		}
	} else {
		// 兼容旧数据：Resource.FileID 为 0 时，尝试通过 VideoIndexFile.DirName 找到对应的 VideoFile 并更新
		var videoIndex model.VideoIndexFile
		if err := db.Where("resource_id = ?", resourceId).First(&videoIndex).Error; err == nil && videoIndex.DirName != "" {
			result := db.Model(&model.VideoFile{}).Where("dir_name = ? AND status != ?", videoIndex.DirName, model.FileStatusReady).
				Update("status", model.FileStatusReady)
			if result.Error != nil {
				utils.ErrorLog(fmt.Sprintf("【警告】通过DirName更新VideoFile状态失败, DirName=%s", videoIndex.DirName), "transcoding", result.Error.Error())
			} else if result.RowsAffected > 0 {
				utils.InfoLog(fmt.Sprintf("【VideoFile状态更新】DirName=%s 状态设为 FileStatusReady(4), 影响行数=%d", videoIndex.DirName, result.RowsAffected), "transcoding")

				// 同时更新 Resource.FileID（补充旧数据）
				if vf, err := findVideoFileByDirName(db, videoIndex.DirName); err == nil && vf != nil {
					db.Model(&model.Resource{}).Where("id = ? AND file_id = 0", resourceId).Update("file_id", vf.ID)
					utils.InfoLog(fmt.Sprintf("【Resource关联更新】ResourceID=%d 设置 FileID=%d", resourceId, vf.ID), "transcoding")
				}
			}
		}
	}
}

// 注册转码进程
func registerTranscodingProcess(videoID, resourceID uint, cmd *exec.Cmd, cancelFunc context.CancelFunc, outputDir string) {
	processMapMutex.Lock()
	defer processMapMutex.Unlock()

	process := TranscodingProcess{
		VideoID:    videoID,
		ResourceID: resourceID,
		PID:        cmd.Process.Pid,
		Cmd:        cmd,
		CancelFunc: cancelFunc,
		OutputDir:  outputDir,
	}

	transcodingProcesses[videoID] = append(transcodingProcesses[videoID], process)
	utils.InfoLog(fmt.Sprintf("【注册转码进程】VideoID=%d, ResourceID=%d, PID=%d", videoID, resourceID, cmd.Process.Pid), "transcoding")
}

// 注销转码进程
func unregisterTranscodingProcess(videoID uint, pid int) {
	processMapMutex.Lock()
	defer processMapMutex.Unlock()

	processes, exists := transcodingProcesses[videoID]
	if !exists {
		return
	}

	// 按 PID 精确过滤，避免同一 ResourceID 的多清晰度进程被提前全部移除
	newProcesses := make([]TranscodingProcess, 0)
	for _, p := range processes {
		if p.PID != pid {
			newProcesses = append(newProcesses, p)
		}
	}

	if len(newProcesses) == 0 {
		delete(transcodingProcesses, videoID)
		utils.InfoLog(fmt.Sprintf("【注销转码进程】VideoID=%d 所有进程已清理", videoID), "transcoding")
	} else {
		transcodingProcesses[videoID] = newProcesses
		utils.InfoLog(fmt.Sprintf("【注销转码进程】VideoID=%d, PID=%d", videoID, pid), "transcoding")
	}
}

func cleanupTranscodedFilesInOutputDir(outputDir string) error {
	files, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		// 保留原始上传文件和封面文件，避免误删共享源文件
		if strings.HasPrefix(fileName, "upload.") || fileName == "cover.jpg" {
			continue
		}
		filePath := filepath.Join(outputDir, fileName)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// 停止视频的所有转码进程并清理文件
func StopTranscodingAndCleanup(videoID uint) error {
	processMapMutex.Lock()
	processes, exists := transcodingProcesses[videoID]
	if !exists || len(processes) == 0 {
		processMapMutex.Unlock()
		utils.InfoLog(fmt.Sprintf("【停止转码】VideoID=%d 没有正在运行的转码进程", videoID), "transcoding")
		return nil
	}

	// 复制一份进程列表，避免在处理过程中持有锁
	processesCopy := make([]TranscodingProcess, len(processes))
	copy(processesCopy, processes)
	delete(transcodingProcesses, videoID)
	processMapMutex.Unlock()

	utils.InfoLog(fmt.Sprintf("【停止转码】VideoID=%d 找到%d个转码进程，准备停止", videoID, len(processesCopy)), "transcoding")

	cleanedDirs := make(map[string]struct{})
	for _, process := range processesCopy {
		// 取消context（如果有）
		if process.CancelFunc != nil {
			process.CancelFunc()
			utils.InfoLog(fmt.Sprintf("【取消Context】ResourceID=%d", process.ResourceID), "transcoding")
		}

		// 杀死进程
		if process.Cmd != nil && process.Cmd.Process != nil {
			pid := process.Cmd.Process.Pid
			err := process.Cmd.Process.Kill()
			if err != nil {
				utils.ErrorLog(fmt.Sprintf("【杀死进程失败】PID=%d", pid), "transcoding", err.Error())
			} else {
				utils.InfoLog(fmt.Sprintf("【杀死进程成功】PID=%d", pid), "transcoding")
			}
		}

		// 清理转码产物（保留 upload.* 原始文件）
		if process.OutputDir != "" {
			if _, alreadyCleaned := cleanedDirs[process.OutputDir]; alreadyCleaned {
				continue
			}
			cleanedDirs[process.OutputDir] = struct{}{}

			err := cleanupTranscodedFilesInOutputDir(process.OutputDir)
			if err != nil {
				utils.ErrorLog(fmt.Sprintf("【清理转码产物失败】%s", process.OutputDir), "transcoding", err.Error())
			} else {
				utils.InfoLog(fmt.Sprintf("【清理转码产物完成】%s", process.OutputDir), "transcoding")
			}
		}
	}

	utils.InfoLog(fmt.Sprintf("【停止转码完成】VideoID=%d", videoID), "transcoding")
	return nil
}

// HasTranscodingProcess 检查指定VideoID是否有正在运行的转码进程
func HasTranscodingProcess(videoID uint) bool {
	processMapMutex.RLock()
	defer processMapMutex.RUnlock()
	processes, exists := transcodingProcesses[videoID]
	return exists && len(processes) > 0
}

// GetTranscodingProcessCount 获取指定VideoID正在运行的转码进程数量
func GetTranscodingProcessCount(videoID uint) int {
	processMapMutex.RLock()
	defer processMapMutex.RUnlock()
	processes, exists := transcodingProcesses[videoID]
	if !exists {
		return 0
	}
	return len(processes)
}
