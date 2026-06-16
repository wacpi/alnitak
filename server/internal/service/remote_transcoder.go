package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"interastral-peace.com/alnitak/internal/domain/dto"
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

func NewRemoteTranscoder() *RemoteTranscoder {
	return &RemoteTranscoder{
		rdb: global.Redis.RawClient(),
	}
}

// Enqueue 将转码任务序列化为 JSON 并推入 Redis Stream，
// 同时启动协程异步轮询进度。
func (t *RemoteTranscoder) Enqueue(ctx context.Context, info *dto.TranscodingInfo) error {
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
			if status == "completed" || status == "failed" {
				utils.InfoLog(fmt.Sprintf("【远程转码完成】ResourceID=%d status=%s", info.ResourceID, status), "transcoding")
				return
			}
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
