package dto

type DanmakuReq struct {
	Vid   uint    `json:"vid" form:"vid"`
	Part  uint    `json:"part" form:"part"`
	Time  float32 `json:"time" form:"time"`
	Type  int     `json:"type" form:"type"`
	Color string  `json:"color" form:"color"`
	Text  string  `json:"text" form:"text"`
}
