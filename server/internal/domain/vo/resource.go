package vo

import (
	"time"

	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/global"
)

type ResourceResp struct {
	ID        uint      `json:"id"`
	ShortID   string    `json:"shortId"`
	CreatedAt time.Time `json:"createdAt"`
	Vid       uint      `json:"vid"`
	Title     string    `json:"title"`
	Duration    int    `json:"duration"` // 秒
	Status      int    `json:"status"`
	StatusName  string `json:"statusName,omitempty"`
	PlayURL     string `json:"playUrl,omitempty"` // 相对路径入口，需配合 play/grant token
	FileID    uint      `json:"fileId"`    // 关联的视频文件ID（全局去重用）
	Uid       uint      `json:"uid"`       // 上传者ID
	SortOrder int       `json:"sortOrder"` // 排序序号
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
		ID:         resource.ID,
		ShortID:    resource.ShortID,
		CreatedAt:  resource.CreatedAt,
		Vid:        resource.Vid,
		Title:      resource.Title,
		Duration:   resource.Duration,
		Status:     resource.Status,
		StatusName: global.ResourceStatusName(resource.Status),
		PlayURL:    "/api/v1/play/" + resource.ShortID,
		FileID:     resource.FileID,
		Uid:        resource.Uid,
		SortOrder:  resource.SortOrder,
	}
}
