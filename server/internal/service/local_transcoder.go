package service

import (
	"context"
	"fmt"

	"interastral-peace.com/alnitak/internal/domain/dto"
)

// LocalTranscoder 包装现有的 TranscodeService，在本地进程内执行转码。
// 这是 "mode=local" 的默认实现，行为与重构前完全一致。
type LocalTranscoder struct {
	inner *TranscodeService
}

func NewLocalTranscoder() *LocalTranscoder {
	return &LocalTranscoder{inner: GetTranscoder()}
}

// Enqueue 启动 goroutine 执行转码，立即返回。
// 与原来调用方写 `go VideoTransCoding(info)` 等效。
func (t *LocalTranscoder) Enqueue(ctx context.Context, info *dto.TranscodingInfo) error {
	if info == nil {
		return fmt.Errorf("transcoding info is nil")
	}
	go t.inner.ProcessVideo(ctx, info)
	return nil
}

// GetProgress 从 TranscodeService 的内存进度表查询指定 resource 的转码进度。
func (t *LocalTranscoder) GetProgress(ctx context.Context, resourceID uint) (*TranscodingProgress, error) {
	state := t.inner.getResourceProgress(resourceID)
	if state == nil {
		return nil, nil
	}

	qualities := make([]QualityProgress, 0, len(state.Details))
	var total float64
	for _, item := range state.Details {
		qualities = append(qualities, QualityProgress{
			Quality:  item.Quality,
			Progress: item.Progress,
			Status:   item.Status,
		})
		total += item.Progress
	}

	overall := float64(0)
	if len(qualities) > 0 {
		overall = total / float64(len(qualities))
	}

	status := "processing"
	uploadStatus := state.UploadStatus
	if uploadStatus == "success" || uploadStatus == "local" {
		status = "success"
	} else if uploadStatus == "fail" {
		status = "fail"
	}

	var upload *UploadProgress
	if state.UploadStatus != "" {
		upload = &UploadProgress{
			OssType:  state.UploadOSS,
			Progress: state.UploadProgress,
			Status:   state.UploadStatus,
		}
	}

	return &TranscodingProgress{
		ResourceID:      resourceID,
		OverallProgress: overall,
		Status:          status,
		Qualities:       qualities,
		Upload:          upload,
	}, nil
}

// Cancel 通过 TranscodeService 中止指定 video 的所有转码进程并清理产物。
func (t *LocalTranscoder) Cancel(ctx context.Context, videoID uint) error {
	return t.inner.StopTranscodingAndCleanup(videoID)
}
