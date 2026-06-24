package model

import "gorm.io/gorm"

type Resource struct {
	gorm.Model
	Vid       uint    `gorm:"comment:所属视频;index"`
	Uid       uint    `gorm:"comment:所属用户;index"`
	Title     string  `gorm:"type:varchar(255);comment:分P使用的标题"`
	CodecName string  `gorm:"type:varchar(10);comment:视频编码名称"`
	Duration  int `gorm:"comment:视频时长秒;default:0"`
	Status    int     `gorm:"comment:审核状态;not null;index"`
	// 对外可见性：0=隐藏(处理中/排队中), 1=可见(转码完成+已上线)
	// 与 Status 不同，VisibleStatus 仅控制前端是否展示，
	// 新分P 加到已公开视频时保持隐藏，转码完成后改为可见
	VisibleStatus int  `gorm:"type:tinyint(1);default:0;comment:对外可见性:0隐藏,1可见;index"`
	ShortID   string  `gorm:"type:varchar(16);comment:短ID;uniqueIndex" json:"shortId"`

	// 排序字段（值越小越靠前，类似B站默认按上传顺序）
	SortOrder int `gorm:"comment:排序序号;default:0;index"`

	// 全局去重相关字段
	FileID uint `gorm:"comment:关联的视频文件ID;index"`

	// 审核冲突关联字段
	ConflictResourceID uint   `gorm:"comment:冲突稿件ID（驳回时设置）;index"`
	ConflictReason     string `gorm:"type:varchar(500);comment:冲突原因"`
}

func (table *Resource) TableName() string {
	return "resource"
}
