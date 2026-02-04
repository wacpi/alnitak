package dto

type CreatePGCReq struct {
	PGCType   int          `json:"pgc_type" binding:"required"`
	Title     string       `json:"title" binding:"required"`
	Cover     string       `json:"cover" binding:"required"`
	Desc      string       `json:"desc"`
	Year      int          `json:"year"`
	Area      string       `json:"area"`
	Rating    float64      `json:"rating"`
	IsOngoing bool         `json:"is_ongoing"`
	Episodes  []EpisodeReq `json:"episodes" binding:"required"`
}

type EpisodeReq struct {
	EpisodeNumber int     `json:"episode_number" binding:"required"`
	Title         string  `json:"title"`
	VID           uint    `json:"vid" binding:"required"`
	Duration      float64 `json:"duration"`
	PublishTime   string  `json:"publish_time"`
}

type UpdatePGCReq struct {
	PGCID     uint    `json:"pgc_id" binding:"required"`
	Title     string  `json:"title"`
	Cover     string  `json:"cover"`
	Desc      string  `json:"desc"`
	Year      int     `json:"year"`
	Area      string  `json:"area"`
	Rating    float64 `json:"rating"`
	IsOngoing bool    `json:"is_ongoing"`
}

type PGCListReq struct {
	Page      int    `form:"page" binding:"required,min=1"`
	PageSize  int    `form:"page_size" binding:"required,min=1,max=100"`
	PGCType   int    `form:"pgc_type"`
	Status    int    `form:"status"`
	Keyword   string `form:"keyword"`
	Year      int    `form:"year"`
	Area      string `form:"area"`
	IsOngoing bool   `form:"is_ongoing"`
}

type EpisodeListReq struct {
	PGCID    uint `form:"pgc_id" binding:"required"`
	Page     int  `form:"page" binding:"required,min=1"`
	PageSize int  `form:"page_size" binding:"required,min=1,max=100"`
}
