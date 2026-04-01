package model

import "gorm.io/gorm"

type Video struct {
	gorm.Model
	Title       string  `gorm:"type:varchar(255);comment:标题;not null;index"`
	Cover       string  `gorm:"type:varchar(255);comment:封面图;not null"`      // 修复typo: cmment -> comment
	Desc        string  `gorm:"type:varchar(200);comment:视频简介;default:什么都没有~"`
	Uid         uint    `gorm:"comment:用户ID;not null;index"`
	Copyright   bool    `gorm:"comment:是否为原创;not null"`
	Clicks      int64   `gorm:"comment:点击量;default:0"`
	Status      int     `gorm:"comment:审核状态;not null;index"`
	PGCAttached bool    `gorm:"column:pgc_attached;comment:是否被PGC剧集绑定;not null;default:0;index"`
	PartitionId uint    `gorm:"comment:分区ID"`
	Tags        string  `gorm:"type:varchar(100);comment:标签冗余(CSV,便于搜索);"`
	Duration    int     `gorm:"comment:视频时长秒;default:0"`
	Shares      int64   `gorm:"comment:分享数;default:0"`
	ShortID     string  `gorm:"type:varchar(16);comment:短ID;uniqueIndex" json:"shortId"`

	Author User `gorm:"-"`
}

func (table *Video) TableName() string {
	return "video"
}
