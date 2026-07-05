package dto

import "interastral-peace.com/alnitak/internal/domain/types"

type VideoListReq struct {
	Page     int
	PageSize int
}

type VideoFileReq struct {
	Hash              string `json:"hash"`
	FileID            uint   `json:"fileID"`
	Size              int64  `json:"size"`
	ReplaceResourceID uint   `json:"replaceResourceID"` // >0 表示替换该旧资源
}

type VideoCheckResp struct {
	Chunks []int `json:"chunks"`
	FileID uint  `json:"fileID"`
}

type ReviewListReq struct {
	Page     int
	PageSize int
}

type UploadVideoReq struct {
	Vid         uint   `json:"vid"`
	Title       string `json:"title"`
	Cover       string `json:"cover"`
	Desc        string `json:"desc"`
	Copyright   types.CopyrightType `json:"copyright"`
	Tags        string `json:"tags"`
	PartitionId uint   `json:"partitionId"`
}

type EditVideoReq struct {
	Vid   uint   `json:"vid"`
	Title string `json:"title"`
	Cover string `json:"cover"`
	Desc  string `json:"desc"`
	Tags  string `json:"tags"`
}

type SearchVideoReq struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	KeyWords  string `json:"keywords"`
	Sort      string `json:"sort"`
	TimeRange string `json:"timeRange"`
}
