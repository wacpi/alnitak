package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
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
	header := make([]byte, 8)

parseLoop:
	for offset < fileSize {
		_, err := file.ReadAt(header, offset)
		if err != nil {
			break
		}

		// 解析 box size（4 字节大端）
		boxSize := int64(uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3]))
		boxType := string(header[4:8])

		// 处理扩展 size（size == 1 表示使用 64 位 size）
		if boxSize == 1 {
			extHeader := make([]byte, 8)
			file.ReadAt(extHeader, offset+8)
			boxSize = int64(uint64(extHeader[0])<<56 | uint64(extHeader[1])<<48 | uint64(extHeader[2])<<40 | uint64(extHeader[3])<<32 |
				uint64(extHeader[4])<<24 | uint64(extHeader[5])<<16 | uint64(extHeader[6])<<8 | uint64(extHeader[7]))
		}

		// size == 0 表示 box 延伸到文件末尾
		if boxSize == 0 {
			boxSize = fileSize - offset
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
	Cmd        *exec.Cmd
	CancelFunc context.CancelFunc
	OutputDir  string
}

// 全局转码并发控制
var (
	transcodingSemaphore chan struct{} // 信号量，控制同时转码的任务数
	semaphoreOnce        sync.Once
	gpuAvailable         = true                                // GPU是否可用
	gpuCheckMutex        sync.RWMutex                          // 保护GPU状态的读写锁
	gpuFailCount         = 0                                   // GPU连续失败次数
	maxGpuFailCount      = 3                                   // 最大允许GPU失败次数
	transcodingProcesses = make(map[uint][]TranscodingProcess) // key: videoID, value: 该视频的所有转码进程
	processMapMutex      sync.RWMutex                          // 保护进程映射的读写锁
)

// 初始化转码并发控制（根据CPU核心数或配置）
func initTranscodingSemaphore() {
	semaphoreOnce.Do(func() {
		// 默认允许2个转码任务并发（可根据实际服务器配置调整）
		maxConcurrentTranscoding := 2
		if global.Config.Transcoding.UseGpu {
			// GPU模式下可以稍微增加并发数
			maxConcurrentTranscoding = 3
		}
		transcodingSemaphore = make(chan struct{}, maxConcurrentTranscoding)
		utils.InfoLog(fmt.Sprintf("【转码并发控制初始化】最大并发数=%d", maxConcurrentTranscoding), "transcoding")
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

	return &transcodingInfo, nil
}

func VideoTransCoding(transcodingInfo *dto.TranscodingInfo) {
	// 实例级音频编码状态（每次转码调用独立，避免多视频并发冲突）
	var audioEncoded bool
	var audioMu sync.Mutex

	// 初始化并发控制
	initTranscodingSemaphore()

	var wg sync.WaitGroup
	targets := getTranscodingTarget(transcodingInfo)
	wg.Add(len(targets))

	utils.InfoLog(fmt.Sprintf("【转码开始】VideoID=%d, ResourceID=%d, 目标数量=%d",
		transcodingInfo.VideoID, transcodingInfo.ResourceID, len(targets)), "transcoding")

	successCount := 0
	var mu sync.Mutex // 保护successCount

	for _, v := range targets {
		c := v // 处理协程引用循环变量问题
		go func() {
			fileName := c.Resolution + "_" + c.BitrateRate + "_" + c.FpsName

			defer func() {
				if r := recover(); r != nil {
					utils.ErrorLog(fmt.Sprintf("【Goroutine panic】%s: %v", fileName, r), "transcoding", "")
					wg.Done()
				}
			}()

			// 获取转码资源锁（控制并发数）
			transcodingSemaphore <- struct{}{}
			defer func() { <-transcodingSemaphore }() // 释放资源锁

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
					transcodingInfo.InputFile, videoFile, c.Resolution, c.BitrateRate, c.FPS, cancel)

				if videoErr != nil && strings.Contains(videoErr.Error(), "GPU error") {
					handleGPUFailure()
					if ctx.Err() != nil {
						utils.InfoLog(fmt.Sprintf("【跳过CPU降级】%s，转码已取消", fileName), "transcoding")
						wg.Done()
						return
					}
					utils.InfoLog(fmt.Sprintf("【降级CPU编码视频】%s", fileName), "transcoding")
					videoErr = encodeVideoOnly(ctx, transcodingInfo.VideoID, transcodingInfo.ResourceID,
						transcodingInfo.InputFile, videoFile, c.Resolution, c.BitrateRate, c.FPS, cancel)
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
					transcodingInfo.InputFile, videoFile, c.Resolution, c.BitrateRate, c.FPS, cancel)
			}

			if videoErr != nil {
				utils.ErrorLog(fmt.Sprintf("【视频编码失败】%s", fileName), "transcoding", videoErr.Error())
				wg.Done()
				return
			}

			// 检查context是否被取消（在视频编码后）
			if ctx.Err() != nil {
				utils.InfoLog(fmt.Sprintf("【转码取消】%s，context已取消", fileName), "transcoding")
				wg.Done()
				return
			}

			// ========== 编码音频（只编码一次）==========
			audioMu.Lock()
			needEncodeAudio := !audioEncoded
			if needEncodeAudio {
				audioEncoded = true
			}
			audioMu.Unlock()

			// 检查context是否被取消（音频编码前）
			if ctx.Err() != nil {
				utils.InfoLog(fmt.Sprintf("【转码取消】%s，context已取消", fileName), "transcoding")
				wg.Done()
				return
			}

			if needEncodeAudio {
				utils.InfoLog(fmt.Sprintf("【编码音频】audio.m4s (码率=%dk, 采样率=%d, 声道=%d)",
					transcodingInfo.AudioBitRate/1000, transcodingInfo.AudioSampleRate, transcodingInfo.AudioChannels), "transcoding")
				if err := encodeAudioOnly(ctx, transcodingInfo.InputFile, audioFile,
					transcodingInfo.AudioBitRate, transcodingInfo.AudioSampleRate, transcodingInfo.AudioChannels); err != nil {
					utils.ErrorLog("【音频编码失败】", "transcoding", err.Error())
					wg.Done()
					return
				}
			}

			// 检查context是否被取消（等待音频前）
			if ctx.Err() != nil {
				utils.InfoLog(fmt.Sprintf("【转码取消】%s，context已取消", fileName), "transcoding")
				wg.Done()
				return
			}

			// 只有执行了音频编码的 goroutine 才能保存数据库
			// 等待音频编码完成（通过检查 audio.m4s 文件存在且大小大于0）
			if !needEncodeAudio {
				// 等待其他 goroutine 完成音频编码
				waitCount := 0
				maxWaitCount := 3000 // 最多等待5分钟
				for {
					audioPath := transcodingInfo.OutputDir + "audio.m4s"
					info, err := os.Stat(audioPath)
					if err == nil && info.Size() > 0 {
						break
					}
					waitCount++
					if waitCount%100 == 0 {
						var fileSize int64
						if info != nil {
							fileSize = info.Size()
						}
						utils.InfoLog(fmt.Sprintf("【等待音频】%s 已等待 %d 次, 文件存在=%v, 大小=%d",
							fileName, waitCount, err == nil, fileSize), "transcoding")
					}
					if waitCount >= maxWaitCount {
						utils.ErrorLog(fmt.Sprintf("【等待音频超时】%s 放弃等待", fileName), "transcoding", "")
						return
					}
					time.Sleep(100 * time.Millisecond)
				}
			}

			// ========== 保存到数据库 ==========
			width, height, bandwidth, frameRate := parseQualityInfo(fileName)

			// 从 fMP4 文件提取实际的 moov box 范围
			videoFilePath := transcodingInfo.OutputDir + fileName + "_video.m4s"
			audioFilePath := transcodingInfo.OutputDir + "audio.m4s"

			videoInitRange, videoIndexRange, _ := getMP4InitRange(videoFilePath)
			audioInitRange, audioIndexRange, _ := getMP4InitRange(audioFilePath)

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
				VideoCodec:      "avc1.640028",
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
				utils.ErrorLog(fmt.Sprintf("【保存索引失败】%s", fileName), "transcoding", err.Error())
				wg.Done()
				return
			}

			utils.InfoLog(fmt.Sprintf("【成功】%s 转码完成, video=%s, audio=%s",
				fileName, indexFile.VideoFile, indexFile.AudioFile), "transcoding")

			mu.Lock()
			successCount++
			mu.Unlock()

			wg.Done()
		}()
	}

	wg.Wait()

	utils.InfoLog(fmt.Sprintf("【所有转码任务完成】成功=%d, 总数=%d", successCount, len(targets)), "transcoding")

	// 【关键】检查是否所有任务都失败了（可能是被用户取消）
	// 如果成功数为0，说明转码被取消或全部失败，不再继续后续处理
	if successCount == 0 {
		utils.InfoLog(fmt.Sprintf("【跳过后续处理】VideoID=%d 所有转码任务均失败或被取消", transcodingInfo.VideoID), "transcoding")
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
	} else {
		utils.InfoLog("【跳过OSS上传】使用本地存储", "transcoding")
	}

	// 更新状态
	utils.InfoLog(fmt.Sprintf("【调用completeTransCoding】VideoID=%d, ResourceID=%d, status=WAITING_REVIEW",
		transcodingInfo.VideoID, transcodingInfo.ResourceID), "transcoding")
	completeTransCoding(transcodingInfo.VideoID, transcodingInfo.ResourceID, global.WAITING_REVIEW, transcodingInfo.OriginalVideoStatus)
}

// 获取宽度支持的最大分辨率
func getWidthRes(width int) int {
	//1920*1080
	if width >= 1920 {
		return 1080
	}
	// 1280*720
	if width >= 1280 {
		return 720
	}
	// 720*480
	if width >= 720 {
		return 480
	}
	return 360
}

// 获取高度支持的最大分辨率
func getHeigthRes(height int) int {
	//1920*1080
	if height >= 1080 {
		return 1080
	}
	// 1280*720
	if height >= 720 {
		return 720
	}
	// 720*480
	if height >= 480 {
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
			return "30000/1001", ""
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
				return "30000/1001", "60000/1001"
			}
		}
	}

	// 30-60fps之间，统一使用30帧
	return "30000/1001", ""
}

// 获取转码目标
func getTranscodingTarget(videoInfo *dto.TranscodingInfo) []TranscodingTarget {
	targets := make([]TranscodingTarget, 0)
	maxRresolution := utils.Max(getWidthRes(videoInfo.Width), getHeigthRes(videoInfo.Height))

	switch maxRresolution {
	case 1080:
		if global.Config.Transcoding.Generate1080p60 && videoInfo.FPS60 != "" {
			targets = append(targets, TranscodingTarget{Resolution: "1920x1080", BitrateRate: "6000k", FPS: videoInfo.FPS60, FpsName: "60"})
		}
		targets = append(targets, TranscodingTarget{Resolution: "1920x1080", BitrateRate: "3000k", FPS: videoInfo.FPS30, FpsName: "30"})
		fallthrough
	case 720:
		targets = append(targets, TranscodingTarget{Resolution: "1280x720", BitrateRate: "2000k", FPS: videoInfo.FPS30, FpsName: "30"})
		fallthrough
	case 480:
		targets = append(targets, TranscodingTarget{Resolution: "854x480", BitrateRate: "900k", FPS: videoInfo.FPS30, FpsName: "30"})
		fallthrough
	case 360:
		targets = append(targets, TranscodingTarget{Resolution: "640x360", BitrateRate: "500k", FPS: videoInfo.FPS30, FpsName: "30"})
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

// SegmentBase模式：编码视频（CPU）
// 生成 fragmented MP4 (fMP4) 格式，包含 mvex box，用于 DASH SegmentBase
func encodeVideoOnly(ctx context.Context, videoID, resourceID uint, inputFile, outputFile, quality, rate, fps string, cancelFunc context.CancelFunc) error {
	// 解析帧率，支持 "24000/1001" 或 "30" 格式
	fpsFloat := parseFPS(fps)
	gopSize := int(math.Round(fpsFloat * 5)) // 每5秒一个关键帧，与B站和fragment对齐
	if gopSize < 1 {
		gopSize = 150 // 默认值（30fps * 5s）
	}
	gopSizeStr := strconv.Itoa(gopSize)
	// bufsize = 2 * maxrate，确保ABR码率波动可控
	bufsize := strings.TrimSuffix(rate, "k")
	if bv, err := strconv.Atoi(bufsize); err == nil {
		bufsize = strconv.Itoa(bv*2) + "k"
	} else {
		bufsize = "4000k"
	}
	scaleFilter := fmt.Sprintf("scale=%s:flags=lanczos", quality)

	command := []string{
		"-i", inputFile,
		"-filter_complex", fmt.Sprintf("[0:v]setpts=PTS-STARTPTS,%s", scaleFilter),
		"-an",
		"-c:v", "libx264",
		"-crf", "20",
		"-preset", "slow",
		"-tune", "film",
		"-profile:v", "high",
		"-pix_fmt", "yuv420p",
		"-bf", "0", // 禁用 B 帧：fMP4 SegmentBase + audio-files 双流模式下，
		// B 帧的非单调 PTS 在 fragment 边界会导致 mpv demuxer AV desync
		"-b:v", rate,
		"-maxrate", rate, // 峰值码率限制，防止复杂场景码率飙升
		"-bufsize", bufsize, // VBV缓冲区大小
		"-r", fps,
		"-g", gopSizeStr, // 每5秒一个关键帧，与B站和fragment对齐
		"-keyint_min", gopSizeStr, // 最小关键帧间距=最大，确保关键帧序列完全规律
		"-sc_threshold", "0",
		"-vsync", "cfr",
		"-f", "mp4",
		"-frag_duration", "5000000", // 每5秒一个fragment（微秒），与关键帧对齐，模仿B站
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof+dash+global_sidx",
		"-y", outputFile,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", command...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	// 打印完整命令
	utils.InfoLog(fmt.Sprintf("【CPU编码命令】ffmpeg %s", strings.Join(command, " ")), "transcoding")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("视频编码启动失败: %s, stderr: %s", err.Error(), stderr.String())
	}

	registerTranscodingProcess(videoID, resourceID, cmd, cancelFunc, "")
	defer unregisterTranscodingProcess(videoID, resourceID)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("视频编码失败: %s, stderr: %s", err.Error(), stderr.String())
	}

	return nil
}

// SegmentBase模式：编码视频（GPU）
// 生成 fragmented MP4 (fMP4) 格式，包含 mvex box，用于 DASH SegmentBase
func encodeVideoOnlyGPU(ctx context.Context, videoID, resourceID uint, inputFile, outputFile, quality, rate, fps string, cancelFunc context.CancelFunc) error {
	// 解析帧率，支持 "24000/1001" 或 "30" 格式
	fpsFloat := parseFPS(fps)
	gopSize := int(math.Round(fpsFloat * 5)) // 每5秒一个关键帧，与B站和fragment对齐
	if gopSize < 1 {
		gopSize = 150 // 默认值（30fps * 5s）
	}
	gopSizeStr := strconv.Itoa(gopSize)
	// bufsize = 2 * maxrate，确保ABR码率波动可控
	bufsize := strings.TrimSuffix(rate, "k")
	if bv, err := strconv.Atoi(bufsize); err == nil {
		bufsize = strconv.Itoa(bv*2) + "k"
	} else {
		bufsize = "4000k"
	}
	scaleFilter := fmt.Sprintf("scale=%s:flags=lanczos", quality)

	command := []string{
		"-i", inputFile,
		"-filter_complex", fmt.Sprintf("[0:v]setpts=PTS-STARTPTS,%s", scaleFilter),
		"-an",
		"-c:v", "h264_nvenc",
		"-cq", "22",
		"-preset", "p6",
		"-rc", "vbr_hq",
		"-pix_fmt", "yuv420p",
		"-bf", "0", // 禁用 B 帧：fMP4 SegmentBase + audio-files 双流模式下，
		// B 帧的非单调 PTS 在 fragment 边界会导致 mpv demuxer AV desync
		"-b:v", rate,
		"-maxrate", rate, // 峰值码率限制
		"-bufsize", bufsize, // VBV缓冲区大小
		"-r", fps,
		"-g", gopSizeStr, // 每5秒一个关键帧，与B站和fragment对齐
		"-sc_threshold", "0",
		"-strict_gop", "1", // NVENC强制严格GOP结构，防止关键帧漂移
		"-vsync", "cfr",
		"-f", "mp4",
		"-frag_duration", "5000000", // 每5秒一个fragment（微秒），与关键帧对齐，模仿B站
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof+dash+global_sidx",
		"-y", outputFile,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", command...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	// 打印完整命令
	utils.InfoLog(fmt.Sprintf("【GPU编码命令】ffmpeg %s", strings.Join(command, " ")), "transcoding")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("GPU视频编码启动失败: %s, stderr: %s", err.Error(), stderr.String())
	}

	registerTranscodingProcess(videoID, resourceID, cmd, cancelFunc, "")
	defer unregisterTranscodingProcess(videoID, resourceID)

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
		"-frag_duration", "5000000",
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
	const maxConcurrency = 10 // 最大并发数
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
					time.Sleep(500 * time.Millisecond)
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

	// 查询当前视频状态（需要提前查询,以便决定资源的最终状态）
	var currentVideo model.Video
	tx.Model(&model.Video{}).Where("id = ?", videoId).First(&currentVideo)
	utils.InfoLog(fmt.Sprintf("【事务查询】VideoID=%d 当前status=%d", videoId, currentVideo.Status), "transcoding")

	// 查询当前资源状态
	var currentResource model.Resource
	tx.Model(&model.Resource{}).Where("id = ?", resourceId).First(&currentResource)
	utils.InfoLog(fmt.Sprintf("【事务查询】ResourceID=%d 当前status=%d", resourceId, currentResource.Status), "transcoding")

	// 先检查是否所有资源都转码完成（包括当前这个资源）
	var processingCount int64
	tx.Model(&model.Resource{}).Where("vid = ? and status = ? and id != ?", videoId, global.VIDEO_PROCESSING, resourceId).Count(&processingCount)
	utils.InfoLog(fmt.Sprintf("【事务查询】VideoID=%d 除当前资源外,仍在转码中(status=200)的资源数=%d", videoId, processingCount), "transcoding")

	// 如果视频原本已审核通过（当前status=0，或重新转码记录的原始status=0），资源直接设为审核通过
	// originalVideoStatus >= 0 表示是重新转码场景（普通上传转码为-1）
	if status == global.WAITING_REVIEW && (currentVideo.Status == global.AUDIT_APPROVED || (originalVideoStatus >= 0 && originalVideoStatus == global.AUDIT_APPROVED)) {
		status = global.AUDIT_APPROVED
		utils.InfoLog("【状态修正】视频原本已审核通过,资源status从WAITING_REVIEW(500)改为AUDIT_APPROVED(0)", "transcoding")
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

	// 每个资源转码完成后立即更新对应的 VideoFile 状态为已就绪
	updateVideoFileStatus(currentResource, resourceId)

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
	utils.InfoLog(fmt.Sprintf("【事务执行】更新video表 VideoID=%d status=%d, WHERE条件: status NOT IN (0,2000), 影响行数=%d",
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
func updateVideoFileStatus(currentResource model.Resource, resourceId uint) {
	if currentResource.FileID != 0 {
		result := global.Mysql.Model(&model.VideoFile{}).Where("id = ? AND status != ?", currentResource.FileID, model.FileStatusReady).
			Update("status", model.FileStatusReady)
		if result.Error != nil {
			utils.ErrorLog(fmt.Sprintf("【警告】更新VideoFile状态失败, FileID=%d", currentResource.FileID), "transcoding", result.Error.Error())
		} else {
			utils.InfoLog(fmt.Sprintf("【VideoFile状态更新】FileID=%d 状态设为 FileStatusReady(4), 影响行数=%d", currentResource.FileID, result.RowsAffected), "transcoding")
		}
	} else {
		// 兼容旧数据：Resource.FileID 为 0 时，尝试通过 VideoIndexFile.DirName 找到对应的 VideoFile 并更新
		var videoIndex model.VideoIndexFile
		if err := global.Mysql.Where("resource_id = ?", resourceId).First(&videoIndex).Error; err == nil && videoIndex.DirName != "" {
			result := global.Mysql.Model(&model.VideoFile{}).Where("dir_name = ? AND status != ?", videoIndex.DirName, model.FileStatusReady).
				Update("status", model.FileStatusReady)
			if result.Error != nil {
				utils.ErrorLog(fmt.Sprintf("【警告】通过DirName更新VideoFile状态失败, DirName=%s", videoIndex.DirName), "transcoding", result.Error.Error())
			} else if result.RowsAffected > 0 {
				utils.InfoLog(fmt.Sprintf("【VideoFile状态更新】DirName=%s 状态设为 FileStatusReady(4), 影响行数=%d", videoIndex.DirName, result.RowsAffected), "transcoding")

				// 同时更新 Resource.FileID（补充旧数据）
				var vf model.VideoFile
				if global.Mysql.Where("dir_name = ?", videoIndex.DirName).First(&vf).Error == nil {
					global.Mysql.Model(&model.Resource{}).Where("id = ? AND file_id = 0", resourceId).Update("file_id", vf.ID)
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
		Cmd:        cmd,
		CancelFunc: cancelFunc,
		OutputDir:  outputDir,
	}

	transcodingProcesses[videoID] = append(transcodingProcesses[videoID], process)
	utils.InfoLog(fmt.Sprintf("【注册转码进程】VideoID=%d, ResourceID=%d, PID=%d", videoID, resourceID, cmd.Process.Pid), "transcoding")
}

// 注销转码进程
func unregisterTranscodingProcess(videoID, resourceID uint) {
	processMapMutex.Lock()
	defer processMapMutex.Unlock()

	processes, exists := transcodingProcesses[videoID]
	if !exists {
		return
	}

	// 过滤掉指定的resourceID
	newProcesses := make([]TranscodingProcess, 0)
	for _, p := range processes {
		if p.ResourceID != resourceID {
			newProcesses = append(newProcesses, p)
		}
	}

	if len(newProcesses) == 0 {
		delete(transcodingProcesses, videoID)
		utils.InfoLog(fmt.Sprintf("【注销转码进程】VideoID=%d 所有进程已清理", videoID), "transcoding")
	} else {
		transcodingProcesses[videoID] = newProcesses
		utils.InfoLog(fmt.Sprintf("【注销转码进程】VideoID=%d, ResourceID=%d", videoID, resourceID), "transcoding")
	}
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

		// 清理输出目录
		if process.OutputDir != "" {
			err := os.RemoveAll(process.OutputDir)
			if err != nil {
				utils.ErrorLog(fmt.Sprintf("【删除输出目录失败】%s", process.OutputDir), "transcoding", err.Error())
			} else {
				utils.InfoLog(fmt.Sprintf("【删除输出目录成功】%s", process.OutputDir), "transcoding")
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
