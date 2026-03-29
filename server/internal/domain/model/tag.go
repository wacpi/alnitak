package model

import "gorm.io/gorm"

// Tag 标签主数据（多对多一端）。
type Tag struct {
	gorm.Model
	Name string `gorm:"type:varchar(64);comment:标签名;uniqueIndex;not null"`
}

func (Tag) TableName() string {
	return "tag"
}

// VideoTag 视频与标签关联。
type VideoTag struct {
	VideoID uint `gorm:"primaryKey;comment:视频ID"`
	TagID   uint `gorm:"primaryKey;comment:标签ID"`
}

func (VideoTag) TableName() string {
	return "video_tag"
}
