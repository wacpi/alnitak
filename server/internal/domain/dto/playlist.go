package dto

// 创建合集
type AddPlaylistReq struct {
	Title string `json:"title" binding:"required"`
	Cover string `json:"cover"`
	Desc  string `json:"desc"`
}

// 编辑合集
type EditPlaylistReq struct {
	ID     uint   `json:"id" binding:"required"`
	Title  string `json:"title" binding:"required"`
	Cover  string `json:"cover"`
	Desc   string `json:"desc"`
	IsOpen bool   `json:"isOpen"`
}

// 添加视频到合集
type PlaylistVideoAddReq struct {
	PlaylistID uint   `json:"playlistId" binding:"required"`
	Vids       []uint `json:"vids" binding:"required"`
}

// 从合集移除视频
type PlaylistVideoDelReq struct {
	PlaylistID uint   `json:"playlistId" binding:"required"`
	Vids       []uint `json:"vids" binding:"required"`
}

// 合集视频排序
type PlaylistVideoSortReq struct {
	PlaylistID uint   `json:"playlistId" binding:"required"`
	Vids       []uint `json:"vids" binding:"required"` // 按顺序排列的视频ID数组
}
