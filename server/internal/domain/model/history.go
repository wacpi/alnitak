package model

import "gorm.io/gorm"

type History struct {
	gorm.Model
	Vid              uint    `gorm:"comment:所在视频id;not null"`
	Uid              uint    `gorm:"comment:所属用户ID;not null"`
	Part             uint    `gorm:"comment:分集;default:1"`
	Time             float64 `gorm:"comment:播放进度;not null"`
	Duration         float64 `gorm:"comment:分P总时长;default:0"`
	ResourceShortID  string  `gorm:"type:varchar(16);comment:资源短ID（用于绑定具体资源，不受排序影响）;index"`
}

func (table *History) TableName() string {
	return "history"
}
