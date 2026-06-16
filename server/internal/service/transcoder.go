package service

import (
	"context"

	"interastral-peace.com/alnitak/internal/domain/dto"
)

// Transcoder 转码后端的抽象接口。
// mode=local  时本地进程内执行（封装 TranscodeService）;
// mode=remote 时通过 Redis Streams + OSS 与远程 Worker 池通信。
type Transcoder interface {
	// Enqueue 提交一个转码任务，立即返回。
	//  - local:  启动 goroutine 执行 ProcessVideo
	//  - remote: XADD 到 Redis Stream, 远端 Worker 消费
	Enqueue(ctx context.Context, info *dto.TranscodingInfo) error

	// GetProgress 查询指定 Resource 的转码进度。
	GetProgress(ctx context.Context, resourceID uint) (*TranscodingProgress, error)

	// Cancel 取消指定 Video 下所有正在进行的转码任务。
	Cancel(ctx context.Context, videoID uint) error
}

// QualityProgress 单画质进度
type QualityProgress struct {
	Quality  string  `json:"quality"`
	Progress float64 `json:"progress"` // 0-100
	Status   string  `json:"status"`   // processing / success / fail
}

// TranscodingProgress Resource 级别的整体进度
type TranscodingProgress struct {
	ResourceID      uint              `json:"resourceId"`
	OverallProgress float64           `json:"overallProgress"` // 0-100
	Status          string            `json:"status"`          // processing / success / fail
	Qualities       []QualityProgress `json:"qualities"`
	Upload          *UploadProgress   `json:"upload,omitempty"`
}

// UploadProgress OSS 上传进度
type UploadProgress struct {
	OssType  string  `json:"ossType"`
	Progress float64 `json:"progress"` // 0-100
	Status   string  `json:"status"`   // uploading / success / fail / local
}
