package dto

// PGCRecommendReq PGC 推荐（对齐 B 站按 season 维度做推荐的调用方式）
// - seed_pgc_id: 可选，作为相似推荐的种子（若未传 pgc_type，则以 seed 的 pgc_type 作为过滤条件）
// - pgc_type: 可选，强制指定类型（优先级高于 seed 推断）
type PGCRecommendReq struct {
	Page      int    `form:"page" binding:"required,min=1"`
	PageSize  int    `form:"page_size" binding:"required,min=1,max=50"`
	PGCType   int    `form:"pgc_type"`
	SeedPGCID string `form:"seed_pgc_id"`
	Scene     string `form:"scene"` // home|detail 等，先预留
}

