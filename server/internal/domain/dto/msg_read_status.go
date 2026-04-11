package dto

// ReadStatusSaveReq 上报已读进度
type ReadStatusSaveReq struct {
	Category   string `json:"category"`   // announce / like / reply / at
	ReadUpToId uint   `json:"readUpToId"`
}
