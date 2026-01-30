package model

import "gorm.io/gorm"

type Resource struct {
	gorm.Model
	Vid       uint    `gorm:"comment:所属视频;index"`
	Uid       uint    `gorm:"comment:所属用户;index"`
	Title     string  `gorm:"type:varchar(255);comment:分P使用的标题"`
	CodecName string  `gorm:"type:varchar(10);comment:视频编码名称"`
	Duration  float64 `gorm:"comment:视频时长;default:0"`
	Status    int     `gorm:"comment:审核状态;not null;index"`

	// 全局去重相关字段
	FileID uint `gorm:"comment:关联的视频文件ID;index"`

	// 审核冲突关联字段
	ConflictResourceID uint   `gorm:"comment:冲突稿件ID（驳回时设置）;index"`
	ConflictReason     string `gorm:"type:varchar(500);comment:冲突原因"`
}

func (table *Resource) TableName() string {
	return "resource"
}
