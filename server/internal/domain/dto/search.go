package dto

// SearchKeywordPageReq 文章/用户搜索统一分页请求（json 与 Web SearchVideoType 对齐）
type SearchKeywordPageReq struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	KeyWords  string `json:"keywords"`
	// Sort YouTube 风格：newest / most_viewed / relevance（MVP：relevance 近似 newest）
	Sort      string `json:"sort"`
	// TimeRange YouTube 风格：all / 24h / week / month / year
	TimeRange string `json:"timeRange"`
}
