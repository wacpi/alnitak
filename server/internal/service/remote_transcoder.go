package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/ffmpeg"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

const (
	// Redis Stream 名称，用于转码任务队列
	transcodingQueueStream = "transcoding:queue"
	// Redis Stream 消费者组名
	transcodingQueueGroup = "transcoder"
	// Redis Hash 状态前缀，后接 resourceID
	transcodingStatusPrefix = "transcoding:status:"
	// Pub/Sub 频道：转码取消
	transcodingCancelChannel = "transcoding:cancel"
	// Pub/Sub 频道：转码完成通知
	transcodingCompleteChannel = "transcoding:complete"
	// 进度轮询间隔
	progressPollInterval = 3 * time.Second
)

// RemoteTranscoder 通过 Redis Streams + OSS 与远程 Worker 池通信。
type RemoteTranscoder struct {
	rdb *redis.Client
}

const (
	// 心跳 key 前缀（与 worker 端保持一致）
	workerHeartbeatPrefix = "transcoding:workers:"
)

// WorkerHeartbeat Worker 心跳快照
type WorkerHeartbeat struct {
	Healthy     bool      `json:"healthy"`
	StartedAt   time.Time `json:"startedAt"`
	Uptime      string    `json:"uptime"`
	Concurrency int       `json:"concurrency"`
	JobsActive  int32     `json:"jobsActive"`
	JobsTotal   int64     `json:"jobsTotal"`
	JobsFailed  int64     `json:"jobsFailed"`
	GroupID     string    `json:"groupID"`
	LastSeen    int64     `json:"lastSeen"`
}

// GetAliveWorkers 扫描 Redis 中所有 Worker 的心跳 key，返回在线 Worker 列表。
// 心跳由 Worker 的 heartbeatLoop 每 10s 写入一次，TTL=30s。
func GetAliveWorkers(ctx context.Context) ([]WorkerHeartbeat, error) {
	iter := global.Redis.RawClient().Scan(ctx, 0, workerHeartbeatPrefix+"*", 100).Iterator()
	var workers []WorkerHeartbeat
	for iter.Next(ctx) {
		val, err := global.Redis.RawClient().Get(ctx, iter.Val()).Result()
		if err != nil {
			continue // key 可能在 scan 和 get 之间过期
		}
		var hb WorkerHeartbeat
		if err := json.Unmarshal([]byte(val), &hb); err != nil {
			continue
		}
		workers = append(workers, hb)
	}
	return workers, iter.Err()
}

// HasAliveWorker 快速检查是否有至少一个 Worker 在线。
func HasAliveWorker(ctx context.Context) bool {
	// 用 Exists 检查第一个匹配的 key，比 Scan 快得多
	iter := global.Redis.RawClient().Scan(ctx, 0, workerHeartbeatPrefix+"*", 1).Iterator()
	if iter.Next(ctx) {
		return true
	}
	return false
}

func NewRemoteTranscoder() *RemoteTranscoder {
	return &RemoteTranscoder{
		rdb: global.Redis.RawClient(),
	}
}

// Enqueue 将转码任务序列化为 JSON 并推入 Redis Stream，
// 同时启动协程异步轮询进度。
// 入队前会检查 Worker 容量和队列深度，容量不足时返回错误。
func (t *RemoteTranscoder) Enqueue(ctx context.Context, info *dto.TranscodingInfo) error {
	// 1. 检查 Worker 可用容量
	available, err := getAvailableCapacity(ctx)
	if err != nil {
		zap.L().Warn("Failed to check worker capacity, proceeding anyway", zap.Error(err))
	} else if available <= 0 {
		zap.L().Warn("All workers busy, rejecting enqueue",
			zap.Uint("videoID", info.VideoID),
			zap.Uint("resourceID", info.ResourceID))
		return fmt.Errorf("all transcoding workers are busy, try again later")
	}

	// 2. 检查队列深度上限
	maxDepth := global.Config.Transcoding.MaxQueueDepth
	if maxDepth > 0 {
		streamLen, err := t.rdb.XLen(ctx, transcodingQueueStream).Result()
		if err != nil {
			zap.L().Warn("Failed to check stream length, proceeding anyway", zap.Error(err))
		} else if streamLen >= int64(maxDepth) {
			zap.L().Warn("Transcoding queue full, rejecting enqueue",
				zap.Int64("streamLen", streamLen),
				zap.Int("maxDepth", maxDepth),
				zap.Uint("videoID", info.VideoID),
				zap.Uint("resourceID", info.ResourceID))
			return fmt.Errorf("transcoding queue is full (%d/%d)", streamLen, maxDepth)
		}
	}

	// 3. 从主服务配置填充编码参数，保证 Worker 端与后台设定一致
	info.UseGpu = global.Config.Transcoding.UseGpu
	info.UseH265 = global.Config.Transcoding.UseH265
	info.UseAv1 = global.Config.Transcoding.UseAv1
	info.Generate1080p60 = global.Config.Transcoding.Generate1080p60

	job, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("transcoding job marshal: %w", err)
	}

	if err := t.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: transcodingQueueStream,
		Values: map[string]interface{}{
			"job":        string(job),
			"resourceID": info.ResourceID,
			"videoID":    info.VideoID,
		},
	}).Err(); err != nil {
		return fmt.Errorf("xadd transcoding job: %w", err)
	}

	utils.InfoLog(fmt.Sprintf("【远程转码入队】VideoID=%d ResourceID=%d Stream=%s",
		info.VideoID, info.ResourceID, transcodingQueueStream), "transcoding")

	// 异步轮询进度
	go t.pollProgress(ctx, info)

	return nil
}

// getAvailableCapacity 扫描所有 Worker 心跳，计算总剩余容量（Σ 最大并发 - 当前活跃任务数）。
func getAvailableCapacity(ctx context.Context) (int, error) {
	workers, err := GetAliveWorkers(ctx)
	if err != nil {
		return 0, fmt.Errorf("get alive workers: %w", err)
	}
	if len(workers) == 0 {
		return 0, nil
	}
	var total int
	for _, w := range workers {
		remaining := w.Concurrency - int(w.JobsActive)
		if remaining < 0 {
			remaining = 0
		}
		total += remaining
	}
	return total, nil
}

// pollProgress 在后台轮询 Redis Hash 中的转码进度，直至完成或取消。
func (t *RemoteTranscoder) pollProgress(ctx context.Context, info *dto.TranscodingInfo) {
	statusKey := fmt.Sprintf("%s%d", transcodingStatusPrefix, info.ResourceID)
	ticker := time.NewTicker(progressPollInterval)
	defer ticker.Stop()

	// 订阅取消信号
	pubsub := t.rdb.Subscribe(ctx, transcodingCancelChannel)
	defer pubsub.Close()
	cancelCh := pubsub.Channel()

	timeout := time.After(24 * time.Hour) // 最长等待24小时

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			utils.ErrorLog(fmt.Sprintf("【远程转码超时】ResourceID=%d", info.ResourceID), "transcoding", "")
			return
		case msg, ok := <-cancelCh:
			if ok && msg.Payload == fmt.Sprintf("%d", info.VideoID) {
				utils.InfoLog(fmt.Sprintf("【远程转码取消】VideoID=%d", info.VideoID), "transcoding")
				return
			}
		case <-ticker.C:
			statusMap, err := t.rdb.HGetAll(ctx, statusKey).Result()
			if err != nil || len(statusMap) == 0 {
				continue
			}
			status := statusMap["status"]
			if status == "failed" {
				utils.InfoLog(fmt.Sprintf("【远程转码失败】ResourceID=%d", info.ResourceID), "transcoding")
				GetTranscoder().completeTransaction(ctx, info, global.PROCESSING_FAIL)
				// 清理 Redis 进度哈希，避免垃圾数据堆积
				t.rdb.Del(ctx, statusKey)
				return
			}
			if status == "completed" {
				utils.InfoLog(fmt.Sprintf("【远程转码完成】ResourceID=%d", info.ResourceID), "transcoding")
				t.handleRemoteCompletion(ctx, info, statusMap)
				// 清理 Redis 进度哈希（statusMap 已读取完，删了不影响）
				t.rdb.Del(ctx, statusKey)
				return
			}
		}
	}
}

// handleRemoteCompletion Worker 完成后，从 Redis 读取索引数据并创建 DB 记录。
func (t *RemoteTranscoder) handleRemoteCompletion(ctx context.Context, info *dto.TranscodingInfo, statusMap map[string]string) {
	svc := GetTranscoder()

	// 1. 从 statusMap 中找出所有 idx_{qualityName} 键，解析索引数据
	var indexRecords []model.VideoIndexFile
	for key, val := range statusMap {
		if !strings.HasPrefix(key, "idx_") {
			continue
		}
		qualityName := strings.TrimPrefix(key, "idx_")

		var idx struct {
			VideoInitRange  string `json:"vInit"`
			VideoIndexRange string `json:"vIndex"`
			AudioInitRange  string `json:"aInit"`
			AudioIndexRange string `json:"aIndex"`
			VideoCodec      string `json:"codec"`
		}
		if err := json.Unmarshal([]byte(val), &idx); err != nil {
			utils.ErrorLog("【远程转码】解析索引数据失败", "transcoding",
				fmt.Sprintf("quality=%s err=%v", qualityName, err))
			continue
		}

		width, height, bandwidth, _ := ffmpeg.ParseQualityInfo(qualityName)
		frameRate := ffmpeg.ParseFPS(qualityName[strings.LastIndex(qualityName, "_")+1:])

		videoCodec := idx.VideoCodec
		if videoCodec == "" {
			videoCodec = ffmpeg.DefaultVideoCodec
		}

		audioCodec := ffmpeg.DefaultAudioCodec
		audioBitrate := info.AudioBitRate
		audioSampleRate := info.AudioSampleRate

		indexRecords = append(indexRecords, model.VideoIndexFile{
			ResourceID:      info.ResourceID,
			Quality:         qualityName,
			DirName:         info.DirName,
			TotalDuration:   info.Duration,
			VideoFile:       qualityName + "_video.m4s",
			VideoBandwidth:  bandwidth,
			VideoCodec:      videoCodec,
			Width:           width,
			Height:          height,
			FrameRate:       frameRate,
			VideoInitRange:  idx.VideoInitRange,
			VideoIndexRange: idx.VideoIndexRange,
			AudioFile:       "audio.m4s",
			AudioBandwidth:  audioBitrate,
			AudioCodec:      audioCodec,
			AudioSampleRate: audioSampleRate,
			AudioInitRange:  idx.AudioInitRange,
			AudioIndexRange: idx.AudioIndexRange,
		})
	}

	if len(indexRecords) == 0 {
		utils.ErrorLog("【远程转码】没有有效的索引数据，标记失败", "transcoding",
			fmt.Sprintf("ResourceID=%d", info.ResourceID))
		if err := svc.completeTransaction(ctx, info, global.PROCESSING_FAIL); err != nil {
			utils.ErrorLog("【远程转码】标记失败时事务出错", "transcoding", err.Error())
		}
		return
	}

	// 2. 事务内批量创建 VideoIndexFile 记录（防止部分写入后失败导致脏数据）
	if err := svc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, rec := range indexRecords {
			if err := tx.Create(&rec).Error; err != nil {
				return fmt.Errorf("create index record for %s: %w", rec.Quality, err)
			}
		}
		return nil
	}); err != nil {
		utils.ErrorLog("【远程转码】批量创建索引记录失败", "transcoding", err.Error())
		if err := svc.completeTransaction(ctx, info, global.PROCESSING_FAIL); err != nil {
			utils.ErrorLog("【远程转码】标记失败时事务出错", "transcoding", err.Error())
		}
		return
	}

	// 3. 多音轨记录（远程模式下音频文件在 OSS 上，暂不创建 AudioTrack 记录）
	// TODO: 远程模式需要 Worker 同时上报各音轨的 init/index range

	// 4. 完成事务（更新资源状态）
	utils.InfoLog(fmt.Sprintf("【远程转码】索引创建完成，共 %d 个画质", len(indexRecords)), "transcoding")
	if err := svc.completeTransaction(ctx, info, global.WAITING_REVIEW); err != nil {
		utils.ErrorLog("【远程转码】完成事务失败", "transcoding", err.Error())
	}

	// 5. 清理本地源文件和目录
	// 远程模式下 Worker 从 OSS 拉取源文件进行转码，转码产物也直接写入 OSS，
	// 本地副本（upload.mp4 等）已无用处，删除以释放 VPS 磁盘空间。
	if info.OutputDir != "" {
		if err := os.RemoveAll(info.OutputDir); err != nil {
			utils.ErrorLog("【远程转码】清理本地源文件目录失败", "transcoding",
				fmt.Sprintf("dir=%s err=%v", info.OutputDir, err))
		} else {
			utils.InfoLog(fmt.Sprintf("【远程转码】已清理本地源文件目录: %s", info.OutputDir), "transcoding")
		}
	}
}

// GetProgress 从 Redis Hash 读取转码进度。
func (t *RemoteTranscoder) GetProgress(ctx context.Context, resourceID uint) (*TranscodingProgress, error) {
	statusKey := fmt.Sprintf("%s%d", transcodingStatusPrefix, resourceID)
	statusMap, err := t.rdb.HGetAll(ctx, statusKey).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall %s: %w", statusKey, err)
	}
	if len(statusMap) == 0 {
		return nil, nil
	}

	// 写入的 key 由 Worker 定义，这里做映射
	tp := &TranscodingProgress{}
	if s, ok := statusMap["status"]; ok {
		tp.Status = s
	}
	if p, ok := statusMap["progress"]; ok {
		fmt.Sscanf(p, "%f", &tp.OverallProgress)
	}

	// 各 quality 进度
	if qMap, ok := statusMap["qualities"]; ok {
		_ = json.Unmarshal([]byte(qMap), &tp.Qualities)
	}

	// 上传进度
	if uMap, ok := statusMap["upload"]; ok {
		_ = json.Unmarshal([]byte(uMap), &tp.Upload)
	}

	return tp, nil
}

// Cancel 发布取消信号到 Redis Pub/Sub，Worker 收到后终止对应任务。
func (t *RemoteTranscoder) Cancel(ctx context.Context, videoID uint) error {
	if err := t.rdb.Publish(ctx, transcodingCancelChannel, fmt.Sprintf("%d", videoID)).Err(); err != nil {
		return fmt.Errorf("publish cancel %d: %w", videoID, err)
	}
	return nil
}
