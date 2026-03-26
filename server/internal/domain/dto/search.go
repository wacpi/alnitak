package dto

// SearchKeywordPageReq 视频/文章/用户搜索统一分页请求（json 与 Web SearchVideoType 对齐）
type SearchKeywordPageReq struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	KeyWords string `json:"keywords"`
}
