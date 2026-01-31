package model

import "gorm.io/gorm"

// Playlist 合集表（UP主创建的公开视频合集）
type Playlist struct {
	gorm.Model
	Uid        uint   `gorm:"comment:创建者用户ID;not null;index"`
	Title      string `gorm:"type:varchar(100);comment:合集标题;not null"`
	Cover      string `gorm:"type:varchar(255);comment:封面图"`
	Desc       string `gorm:"type:varchar(500);comment:简介"`
	IsOpen     bool   `gorm:"comment:是否公开;default:true"`
	Status     int    `gorm:"comment:审核状态;default:500;index"`
	VideoCount int    `gorm:"comment:视频数量;default:0"`
	Views      int64  `gorm:"comment:浏览量;default:0"`
	Favorites  int64  `gorm:"comment:收藏数;default:0"`
}

func (table *Playlist) TableName() string {
	return "playlist"
}

// PlaylistVideo 合集-视频关联表
type PlaylistVideo struct {
	gorm.Model
	PlaylistID uint  `gorm:"comment:合集ID;not null;uniqueIndex:idx_playlist_vid"`
	Vid        uint  `gorm:"comment:视频ID;not null;uniqueIndex:idx_playlist_vid"`
	Sort       int64 `gorm:"comment:排序值;default:0;index"`
}

func (table *PlaylistVideo) TableName() string {
	return "playlist_video"
}
