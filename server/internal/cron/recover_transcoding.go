package cron

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/internal/service"
	"interastral-peace.com/alnitak/utils"
)

const (
	transcodingQueueStream    = "transcoding:queue"
	transcodingStatusPrefix   = "transcoding:status:"
)



// RecoverStuckTranscoding 定期恢复卡住的转码任务
//
// 转码任务入队（Enqueue）失败时错误被静默忽略（_ = Enqueue(...)），
// 导致资源状态停留在 VIDEO_PROCESSING 但实际没有 Worker 处理。
// 此函数检查所有处于 VIDEO_PROCESSING 的资源，如果没有对应的
// Redis 进度哈希（transcoding:status:{resourceID}），说明任务从未
// 成功入队，尝试重新入队。
func RecoverStuckTranscoding() {
	utils.InfoLog("开始检查卡住的转码任务", "cron")

	var resources []model.Resource
	global.Mysql.Model(&model.Resource{}).
		Where("status = ?", global.VIDEO_PROCESSING).
		Find(&resources)

	if len(resources) == 0 {
		utils.InfoLog("没有卡住的转码任务", "cron")
		return
	}

	rdb := global.Redis.RawClient()
	ctx := context.Background()

	recovered := 0
	skipped := 0
	errors := 0

	for _, res := range resources {
		redisKey := fmt.Sprintf("%s%d", transcodingStatusPrefix, res.ID)

		// 检查 Redis 中是否有该资源的进度哈希
		exists, err := rdb.Exists(ctx, redisKey).Result()
		if err != nil {
			utils.ErrorLog("检查Redis失败", "cron",
				fmt.Sprintf("ResourceID=%d, err=%v", res.ID, err))
			errors++
			continue
		}

		if exists > 0 {
			// Redis 中有进度记录，说明 pollProgress 在运行或者任务已成功入队
			skipped++
			continue
		}

		// 再检查 Stream 中是否已有该 resourceID 的条目
		// （Enqueue 成功后 Worker 还没开始处理时，Hash 不存在但 Stream 里有消息）
		inStream, err := resourceInStream(ctx, rdb, res.ID)
		if err != nil {
			utils.ErrorLog("检查Stream失败", "cron",
				fmt.Sprintf("ResourceID=%d, err=%v", res.ID, err))
			errors++
			continue
		}
		if inStream {
			skipped++
			continue
		}

		// 没有 Redis 进度记录 + Stream 中没有 → Enqueue 从未成功 → 需要重新入队
		utils.InfoLog(fmt.Sprintf("发现卡住的转码任务，准备重新入队: ResourceID=%d, VideoID=%d", res.ID, res.Vid), "cron")

		if err := reenqueueResource(res); err != nil {
			utils.ErrorLog("重新入队失败", "cron",
				fmt.Sprintf("ResourceID=%d, err=%v", res.ID, err))
			errors++
			continue
		}

		recovered++
	}

	utils.InfoLog(fmt.Sprintf("转码恢复任务完成: 恢复=%d, 跳过(已有进度)=%d, 错误=%d",
		recovered, skipped, errors), "cron")
}

// resourceInStream 检查指定 resourceID 是否已在 Redis Stream 中等待处理。
// 扫描 Stream 所有条目（maxDepth <= 10，最多 10 条），避免在 Worker 
// 还未开始处理（未写进度 Hash）时重复入队。
func resourceInStream(ctx context.Context, rdb *redis.Client, resourceID uint) (bool, error) {
	entries, err := rdb.XRange(ctx, transcodingQueueStream, "-", "+").Result()
	if err != nil {
		return false, err
	}
	target := strconv.FormatUint(uint64(resourceID), 10)
	for _, entry := range entries {
		if rid, ok := entry.Values["resourceID"].(string); ok && rid == target {
			return true, nil
		}
	}
	return false, nil
}

// reenqueueResource 为指定资源重建 TranscodingInfo 并重新入队
func reenqueueResource(res model.Resource) error {
	// 1. 获取 VideoFile 信息（用于构建文件路径）
	var vf model.VideoFile
	if err := global.Mysql.Where("id = ?", res.FileID).First(&vf).Error; err != nil {
		return fmt.Errorf("查找VideoFile失败: %w", err)
	}
	if vf.DirName == "" {
		return fmt.Errorf("VideoFile.DirName 为空, FileID=%d", res.FileID)
	}

	suffix := utils.GetFileSuffix(vf.OriginalName)

	// 2. 如果是远程模式，Worker 会从 OSS 下载文件并自行 ffprobe，
	//    只需提供基础字段即可；本地模式需要探测源文件。
	inputPath := "./upload/video/" + vf.DirName + "/upload" + suffix

	var info *dto.TranscodingInfo

	if global.Config.Transcoding.Mode == "remote" {
		// 远程模式：Worker 下载后自己 probe，只需传基础信息
		info = &dto.TranscodingInfo{
			VideoID:             res.Vid,
			ResourceID:          res.ID,
			InputFile:           inputPath,
			OutputDir:           "./upload/video/" + vf.DirName + "/",
			DirName:             vf.DirName,
			Suffix:              suffix,
			CodecName:           res.CodecName,
			Duration:            float64(res.Duration),
			OriginalVideoStatus: -1, // 普通转码恢复，不是重新审核
		}
	} else {
		// 本地模式：需要完整文件信息
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			return fmt.Errorf("源文件不存在: %s", inputPath)
		}
		probed, err := service.ProcessVideoInfo(inputPath)
		if err != nil {
			return fmt.Errorf("探测视频信息失败: %w", err)
		}
		info = &dto.TranscodingInfo{
			VideoID:             res.Vid,
			ResourceID:          res.ID,
			InputFile:           inputPath,
			OutputDir:           "./upload/video/" + vf.DirName + "/",
			DirName:             vf.DirName,
			Suffix:              suffix,
			Width:               probed.Width,
			Height:              probed.Height,
			Duration:            probed.Duration,
			CodecName:           probed.CodecName,
			FPS:                 probed.FPS,
			FPS30:               probed.FPS30,
			FPS60:               probed.FPS60,
			VideoBitRate:        probed.VideoBitRate,
			AudioBitRate:        probed.AudioBitRate,
			AudioSampleRate:     probed.AudioSampleRate,
			AudioChannels:       probed.AudioChannels,
			AudioStreams:        probed.AudioStreams,
			OriginalVideoStatus: -1,
		}
	}

	// 3. 入队
	if err := service.GetCurrentTranscoder().Enqueue(context.Background(), info); err != nil {
		return fmt.Errorf("Enqueue失败: %w", err)
	}

	utils.InfoLog(fmt.Sprintf("转码任务重新入队成功: ResourceID=%d, VideoID=%d, DirName=%s",
		res.ID, res.Vid, vf.DirName), "cron")

	return nil
}
