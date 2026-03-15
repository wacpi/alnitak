package vo

import (
	"time"

	"interastral-peace.com/alnitak/internal/domain/model"
)

type ResourceResp struct {
	ID        uint      `json:"id"`
	ShortID   string    `json:"shortId"`
	CreatedAt time.Time `json:"createdAt"`
	Vid       uint      `json:"vid"`
	Title     string    `json:"title"`
	Duration  float64   `json:"duration"`
	Status    int       `json:"status"`
	FileID    uint      `json:"fileId"` // 关联的视频文件ID（全局去重用）
	Uid       uint      `json:"uid"`    // 上传者ID
}

// RelatedResourceResp 相同文件的关联稿件信息
type RelatedResourceResp struct {
	ResourceID uint      `json:"resourceId"`
	Vid        uint      `json:"vid"`
	Uid        uint      `json:"uid"`
	Title      string    `json:"title"`
	Status     int       `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	AuthorName string    `json:"authorName"` // 作者名称
}

func ResourceToResourceResp(resource model.Resource) ResourceResp {
	return ResourceResp{
		ID:        resource.ID,
		ShortID:   resource.ShortID,
		CreatedAt: resource.CreatedAt,
		Vid:       resource.Vid,
		Title:     resource.Title,
		Duration:  resource.Duration,
		Status:    resource.Status,
		FileID:    resource.FileID,
		Uid:       resource.Uid,
	}
}
