package dto

type HistoryReq struct {
	Vid      uint
	Part     uint
	Time     float64 // 当前播放进度
	Duration float64 // 当前分P总时长
}
