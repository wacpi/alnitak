package dto

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
	Vid         uint
	Title       string
	Cover       string
	Desc        string
	Copyright   int8  `form:"copyright"`
	Tags        string
	PartitionId uint //分区ID
}

type EditVideoReq struct {
	Vid   uint
	Title string
	Cover string
	Desc  string
	Tags  string
}

type SearchVideoReq struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	KeyWords  string `json:"keywords"`
	Sort      string `json:"sort"`
	TimeRange string `json:"timeRange"`
}
