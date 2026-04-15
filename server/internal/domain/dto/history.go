package dto

type HistoryReq struct {
	Vid               uint    `json:"vid" form:"vid"`
	Part              uint    `json:"part" form:"part"`
	Time              float64 `json:"time" form:"time"`
	Duration          float64 `json:"duration" form:"duration"`
	ResourceShortID   string  `json:"resourceShortId" form:"resourceShortId"`
}
