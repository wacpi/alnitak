package dto

type LikeVideoReq struct {
	Vid uint `json:"vid" form:"vid"`
}

type LikeArticleReq struct {
	Aid uint `json:"aid" form:"aid"`
}
