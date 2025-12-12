package service

import (
	"encoding/json"
	"fmt"
	"io"
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

type TranscodingTarget struct {
	Resolution  string // 分辨率
	BitrateRate string // 码率
	FPS         string // 帧率
	FpsName     string // 帧率名称
}

// 全局转码并发控制
var (
	transcodingSemaphore chan struct{} // 信号量，控制同时转码的任务数
	semaphoreOnce        sync.Once
	gpuAvailable         = true       // GPU是否可用
	gpuCheckMutex        sync.RWMutex // 保护GPU状态的读写锁
	gpuFailCount         = 0          // GPU连续失败次数
	maxGpuFailCount      = 3          // 最大允许GPU失败次数
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
	var transcodingInfo dto.TranscodingInfo
	videoData, err := getVideoInfo(input)
	if err != nil {
		utils.ErrorLog("读取视频信息失败", "transcoding", err.Error())
		return &transcodingInfo, err
	}

	// 计算最大分辨率
	transcodingInfo.Width = videoData.Stream[0].Width
	transcodingInfo.Height = videoData.Stream[0].Height
	transcodingInfo.CodecName = videoData.Stream[0].CodecName

	// 获取视频时长
	transcodingInfo.Duration, _ = strconv.ParseFloat(videoData.Stream[0].Duration, 64)

	// 获取帧率
	transcodingInfo.FPS = videoData.Stream[0].AvgFrameRate
	transcodingInfo.FPS30, transcodingInfo.FPS60 = getFpsInfo(transcodingInfo.FPS)

	return &transcodingInfo, nil
}

func VideoTransCoding(transcodingInfo *dto.TranscodingInfo) {
	// 初始化并发控制
	initTranscodingSemaphore()

	utils.InfoLog(fmt.Sprintf("【转码开始】VideoID=%d, ResourceID=%d, 目标数量=%d",
		transcodingInfo.VideoID, transcodingInfo.ResourceID, len(getTranscodingTarget(transcodingInfo))), "transcoding")

	var wg sync.WaitGroup
	targets := getTranscodingTarget(transcodingInfo)
	wg.Add(len(targets))

	successCount := 0
	var mu sync.Mutex // 保护successCount

	for _, v := range targets {
		c := v // 处理协程引用循环变量问题
		go func() {
			// 获取转码资源锁（控制并发数）
			transcodingSemaphore <- struct{}{}
			defer func() { <-transcodingSemaphore }() // 释放资源锁

			fileName := c.Resolution + "_" + c.BitrateRate + "_" + c.FpsName

			utils.InfoLog(fmt.Sprintf("【开始HLS一步式转码】%s", fileName), "transcoding")

			// 智能选择转码方式：GPU优先，失败时自动降级到CPU
			var m3u8File string
			var err error
			useGpu := global.Config.Transcoding.UseGpu && checkGPUAvailable()

			if useGpu {
				utils.InfoLog(fmt.Sprintf("【使用GPU一步式转码】%s", fileName), "transcoding")
				m3u8File, err = pressingVideoGPU(transcodingInfo.InputFile, transcodingInfo.OutputDir, fileName, c.Resolution, c.BitrateRate, c.FPS)

				if err != nil {
					utils.ErrorLog(fmt.Sprintf("【GPU转码失败】%s，尝试切换到CPU", fileName), "transcoding", err.Error())
					handleGPUFailure()

					// GPU失败后尝试使用CPU
					utils.InfoLog(fmt.Sprintf("【降级到CPU一步式转码】%s", fileName), "transcoding")
					m3u8File, err = pressingVideo(transcodingInfo.InputFile, transcodingInfo.OutputDir, fileName, c.Resolution, c.BitrateRate, c.FPS)
				} else {
					// GPU转码成功，重置失败计数
					gpuCheckMutex.Lock()
					if gpuFailCount > 0 {
						gpuFailCount = 0
						utils.InfoLog("【GPU恢复】转码成功，重置失败计数", "transcoding")
					}
					gpuCheckMutex.Unlock()
				}
			} else {
				if global.Config.Transcoding.UseGpu && !checkGPUAvailable() {
					utils.InfoLog(fmt.Sprintf("【使用CPU一步式转码】%s（GPU已禁用）", fileName), "transcoding")
				} else {
					utils.InfoLog(fmt.Sprintf("【使用CPU一步式转码】%s", fileName), "transcoding")
				}
				m3u8File, err = pressingVideo(transcodingInfo.InputFile, transcodingInfo.OutputDir, fileName, c.Resolution, c.BitrateRate, c.FPS)
			}

			if err != nil {
				utils.ErrorLog(fmt.Sprintf("【HLS转码失败】%s", fileName), "transcoding", err.Error())
				wg.Done()
				return
			}

			utils.InfoLog(fmt.Sprintf("【HLS转码完成】%s，保存到数据库", fileName), "transcoding")

			// 读取m3u8写入数据库
			err = saveM3u8File(transcodingInfo, fileName, m3u8File)
			if err != nil {
				utils.ErrorLog(fmt.Sprintf("【保存m3u8失败】%s", fileName), "transcoding", err.Error())
				wg.Done()
				return
			}

			utils.InfoLog(fmt.Sprintf("【成功】%s 转码完成", fileName), "transcoding")

			// 删除临时m3u8文件（TS分段文件保留在磁盘）
			os.Remove(m3u8File)

			mu.Lock()
			successCount++
			mu.Unlock()

			wg.Done()
		}()
	}

	wg.Wait()

	utils.InfoLog(fmt.Sprintf("【所有转码任务完成】成功=%d, 总数=%d", successCount, len(targets)), "transcoding")

	// 上传oss - 添加panic恢复
	defer func() {
		if r := recover(); r != nil {
			utils.ErrorLog("【OSS上传panic】", "transcoding", fmt.Sprintf("%v", r))
			utils.InfoLog("【调用completeTransCoding】status=PROCESSING_FAIL（OSS panic）", "transcoding")
			completeTransCoding(transcodingInfo.VideoID, transcodingInfo.ResourceID, global.PROCESSING_FAIL)
		}
	}()

	if global.Config.Storage.OssType != "local" {
		utils.InfoLog(fmt.Sprintf("【开始上传OSS】OssType=%s", global.Config.Storage.OssType), "transcoding")

		files, err := os.ReadDir(transcodingInfo.OutputDir)
		if err != nil {
			utils.ErrorLog("读取视频文件夹失败", "oss", err.Error())
			utils.InfoLog("【调用completeTransCoding】status=PROCESSING_FAIL（OSS失败）", "transcoding")
			completeTransCoding(transcodingInfo.VideoID, transcodingInfo.ResourceID, global.PROCESSING_FAIL)
			return
		}

		// 并发上传文件
		uploadCount := uploadFilesToOSS(transcodingInfo.DirName, transcodingInfo.OutputDir, files)
		utils.InfoLog(fmt.Sprintf("【OSS上传完成】成功上传=%d/%d个文件", uploadCount, len(files)), "transcoding")
	} else {
		utils.InfoLog("【跳过OSS上传】使用本地存储", "transcoding")
	}

	// 更新状态
	utils.InfoLog(fmt.Sprintf("【调用completeTransCoding】VideoID=%d, ResourceID=%d, status=WAITING_REVIEW",
		transcodingInfo.VideoID, transcodingInfo.ResourceID), "transcoding")
	completeTransCoding(transcodingInfo.VideoID, transcodingInfo.ResourceID, global.WAITING_REVIEW)
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
		if fps < 30 {
			return avgFrameRate, ""
		}
		if fps >= 60 {
			return "30000/1001", "60000/1001"
		}
	}

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
	cmd := exec.Command("ffprobe", "-i", input, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams")
	out, err := utils.RunCmd(cmd)
	if err != nil {
		return info, err
	}

	// 反序列化
	err = json.Unmarshal(out.Bytes(), &info)
	if err != nil {
		return info, err
	}

	// 提取帧率信息
	if len(info.Stream) > 0 && info.Stream[0].AvgFrameRate != "" {
		// 将帧率转换为浮点数（例如 "30000/1001" 转换为 29.97）
		parts := strings.Split(info.Stream[0].AvgFrameRate, "/")
		if len(parts) == 2 {
			numerator, _ := strconv.Atoi(parts[0])
			denominator, _ := strconv.Atoi(parts[1])
			if denominator != 0 {
				// 将浮点数转换为字符串
				info.Stream[0].RFrameRate = fmt.Sprintf("%.2f", float64(numerator)/float64(denominator))
			}
		}
	}

	return info, nil
}

// CPU一步式HLS转码：边转码边切片 (优化画质 + 音频接近无损重编码)
func pressingVideo(inputFile, outputDir, fileName, quality, rate, fps string) (string, error) {
	outputM3U8 := outputDir + fileName + ".m3u8"
	outputTs := outputDir + fileName + "_%05d.ts"

	// 解析 fps
	fpsInt, err := strconv.Atoi(fps)
	if err != nil {
		fpsInt = 30 // 默认30fps
	}

	// 10秒切片
	sliceSeconds := 10
	gop := fpsInt * sliceSeconds

	// 引入 Lanczos 高质量缩放滤镜
	scaleFilter := fmt.Sprintf("scale=%s:flags=lanczos", quality)

	command := []string{
		"-i", inputFile,

		// --- 视频参数 ---
		// 1. 画质控制：CRF 20 (建议值)
		"-crf", "20",
		// 2. 缩放质量：Lanczos 算法
		"-vf", scaleFilter,
		// 3. 编码效率：slow (同码率下画质更好)
		"-preset", "slow",
		// 4. 心理视觉调优
		"-tune", "film",
		// 5. 去块滤波优化
		"-x264-params", "deblock=-1:-1:keyint-min=" + strconv.Itoa(gop),
		// 6. Profile
		"-profile:v", "high",
		"-b:v", rate,
		"-c:v", "libx264",
		"-r", fps,

		// --- HLS 关键帧对齐 ---
		"-g", strconv.Itoa(gop),
		"-sc_threshold", "0",
		"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", sliceSeconds),
		"-vsync", "cfr",

		// --- 音频参数 (修改部分) ---
		// 行业标准高音质重编码：解决兼容性与时间戳问题，同时保持接近无损的听感
		"-c:a", "aac", // 编码器：AAC (兼容性之王)
		"-b:a", "320k", // 码率：320kbps (AAC的画质上限，听感透明)
		"-ar", "48000", // 采样率：48kHz (视频标准)
		"-ac", "2", // 声道：双声道 (立体声)
		"-af", "aresample=async=1", // 音频滤镜：修正音频时间戳，防止音画不同步

		// --- HLS 切片设置 ---
		"-f", "hls",
		"-hls_time", strconv.Itoa(sliceSeconds),
		"-hls_list_size", "0",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", outputTs,
		"-hls_playlist_type", "vod",

		outputM3U8,
	}

	_, err = utils.RunCmd(exec.Command("ffmpeg", command...))
	if err != nil {
		utils.ErrorLog("HLS转码失败", "transcoding", err.Error())
		return outputM3U8, err
	}

	return outputM3U8, nil
}

// GPU一步式HLS转码：边转码边切片（使用NVIDIA硬件加速 - 优化画质 + 音频接近无损重编码）
func pressingVideoGPU(inputFile, outputDir, fileName, quality, rate, fps string) (string, error) {
	outputM3U8 := outputDir + fileName + ".m3u8"
	outputTs := outputDir + fileName + "_%05d.ts"

	fpsInt, err := strconv.Atoi(fps)
	if err != nil {
		fpsInt = 30
	}

	sliceSeconds := 10
	gop := fpsInt * sliceSeconds

	// 引入 Lanczos 高质量缩放滤镜
	scaleFilter := fmt.Sprintf("scale=%s:flags=lanczos", quality)

	command := []string{
		"-i", inputFile,

		// --- 视频参数 (NVENC) ---
		// 1. 恒定质量控制
		"-cq", "22",
		// 2. 缩放
		"-vf", scaleFilter,
		// 3. 预设：最高质量 p6
		"-preset", "p6",
		// 4. 码率控制模式：VBR_HQ
		"-rc", "vbr_hq",

		"-b:v", rate, // MaxRate
		"-c:v", "h264_nvenc",
		"-r", fps,

		// --- HLS 关键帧对齐 ---
		"-g", strconv.Itoa(gop),
		"-keyint_min", strconv.Itoa(gop),
		"-sc_threshold", "0",
		"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", sliceSeconds),
		"-vsync", "cfr",

		// --- 音频参数 (修改部分) ---
		// 即使是 GPU 视频转码，音频通常也是由 CPU 处理的，参数同上
		"-c:a", "aac",
		"-b:a", "320k",
		"-ar", "48000",
		"-ac", "2",
		"-af", "aresample=async=1", // 关键：防止 GPU 视频编码速度过快导致的音画同步问题

		// --- HLS 切片设置 ---
		"-f", "hls",
		"-hls_time", strconv.Itoa(sliceSeconds),
		"-hls_list_size", "0",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", outputTs,
		"-hls_playlist_type", "vod",

		outputM3U8,
	}

	// 执行命令
	out, err := utils.RunCmd(exec.Command("ffmpeg", command...))
	if err != nil {
		errMsg := err.Error()
		outStr := out.String()

		// 检测是否是GPU相关错误
		if strings.Contains(outStr, "No NVENC capable devices found") ||
			strings.Contains(outStr, "Cannot load nvcuda.dll") ||
			strings.Contains(outStr, "CUDA driver version is insufficient") ||
			strings.Contains(outStr, "h264_nvenc") ||
			strings.Contains(errMsg, "nvenc") {
			utils.ErrorLog("GPU不可用或驱动异常", "transcoding", outStr)
			return outputM3U8, fmt.Errorf("GPU error: %s", outStr)
		}

		utils.ErrorLog("GPU HLS转码失败", "transcoding", errMsg)
		return outputM3U8, err
	}

	return outputM3U8, nil
}

/*
func generateVideoSlices(inputFile, outputDir, outputName string) (string, error) {
	outputM3U8 := outputDir + outputName + ".m3u8"
	outputTs := outputDir + outputName + "_%05d.ts"

	command := []string{
		"-i", inputFile,
		"-c", "copy",
		"-map", "0",
		"-f", "segment",
		"-segment_list", outputM3U8,
		"-segment_time", "10",
		"-segment_list_flags", "+live", // 确保最后一个片段被正确写入
		"-break_non_keyframes", "1",    // 允许在非关键帧处切割，避免丢失尾部内容
		outputTs,
	}

	_, err := utils.RunCmd(exec.Command("ffmpeg", command...))
	if err != nil {
		utils.ErrorLog("生成视频切片失败", "transcoding", err.Error())
		return outputM3U8, err
	}

	return outputM3U8, nil
}
*/

// 保存m3u8文件
func saveM3u8File(transcodingInfo *dto.TranscodingInfo, fileName, m3u8File string) error {
	file, err := os.Open(m3u8File)
	if err != nil {
		utils.ErrorLog("打开m3u8文件失败", "transcoding", err.Error())
		return err
	}

	bytes, err := io.ReadAll(file)
	if err != nil {
		utils.ErrorLog("读取m3u8文件失败", "transcoding", err.Error())
		return err
	}

	file.Close()

	global.Mysql.Create(&model.VideoIndexFile{
		ResourceID: transcodingInfo.ResourceID,
		Quality:    fileName,
		DirName:    transcodingInfo.DirName,
		Content:    string(bytes),
	})

	return nil
}

// 并发上传文件到OSS
func uploadFilesToOSS(dirName, outputDir string, files []os.DirEntry) int {
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

				// 跳过upload.mp4如果配置不上传
				if fileName == "upload.mp4" && !global.Config.Storage.UploadMp4File {
					utils.InfoLog("【OSS跳过】upload.mp4 (配置不上传原文件)", "transcoding")
					results <- false
					continue
				}

				objectKey := "video/" + dirName + "/" + fileName
				filePath := outputDir + fileName

				utils.InfoLog(fmt.Sprintf("【OSS上传中】Worker%d: %d/%d %s", workerID, task.index+1, len(files), fileName), "transcoding")

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
func completeTransCoding(videoId, resourceId uint, status int) error {
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

	// 如果还有其他资源在转码中,当前资源保持VIDEO_PROCESSING状态,不更新
	// 只有当所有资源都转码完成时,才统一更新所有资源状态
	if processingCount > 0 {
		utils.InfoLog(fmt.Sprintf("【跳过更新】还有%d个其他资源在转码中,当前资源保持VIDEO_PROCESSING(200)状态", processingCount), "transcoding")
		// 不更新资源状态,直接提交事务
		if err := tx.Commit().Error; err != nil {
			utils.ErrorLog("【事务失败】提交事务失败", "transcoding", err.Error())
			return err
		}
		utils.InfoLog("【事务提交】成功（资源状态未更新）", "transcoding")
		cache.DelVideoInfo(videoId)
		utils.InfoLog(fmt.Sprintf("【缓存清理】删除VideoID=%d的缓存", videoId), "transcoding")
		utils.InfoLog("========== completeTransCoding 结束 ==========", "transcoding")
		return nil
	}

	// 所有资源都转码完成了,现在统一更新状态
	utils.InfoLog("【判断】所有资源转码已完成,准备统一更新所有资源状态", "transcoding")

	// 如果视频已经审核通过,资源应该直接设为审核通过状态
	if currentVideo.Status == global.AUDIT_APPROVED && status == global.WAITING_REVIEW {
		status = global.AUDIT_APPROVED
		utils.InfoLog("【状态修正】视频已审核通过,资源status从WAITING_REVIEW(500)改为AUDIT_APPROVED(0)", "transcoding")
	}

	// 统一更新所有转码中的资源状态(包括当前资源)
	result := tx.Model(&model.Resource{}).Where("vid = ? and status = ?", videoId, global.VIDEO_PROCESSING).Updates(
		map[string]any{
			"status": status,
		},
	)
	if result.Error != nil {
		tx.Rollback()
		utils.ErrorLog("【事务失败】批量更新资源状态失败", "transcoding", result.Error.Error())
		return result.Error
	}
	utils.InfoLog(fmt.Sprintf("【事务执行】批量更新resource表 VideoID=%d所有资源 status=%d, 影响行数=%d", videoId, status, result.RowsAffected), "transcoding")

	// 所有资源状态已更新完成,现在更新视频状态
	utils.InfoLog("【判断】所有资源转码已完成，准备更新video状态", "transcoding")

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
	} else {
		// 至少有一个资源成功，视频状态设为待审核
		videoStatus = global.WAITING_REVIEW
		utils.InfoLog("【判断】至少一个资源成功，video status设为WAITING_REVIEW(500)", "transcoding")
	}

	// 更新视频状态（不限制为SUBMIT_REVIEW，允许从CREATED_VIDEO等状态更新）
	videoResult := tx.Model(&model.Video{}).Where("id = ? and status NOT IN (?, ?)", videoId, global.AUDIT_APPROVED, global.REVIEW_FAILED).Updates(
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
