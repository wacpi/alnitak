package dto

type ReviewVideoReq struct {
	Vid                uint
	Status             int
	Remark             string
	ConflictResourceID uint   `json:"conflictResourceId"` // 冲突稿件的资源ID（可选，用于重复内容驳回）
	ConflictReason     string `json:"conflictReason"`     // 冲突原因说明（可选）
}

type ReviewArticleReq struct {
	Aid    uint
	Status int
	Remark string
}

type ReviewPlaylistReq struct {
	ID     uint `json:"id"`
	Status int
	Remark string
}
