package model

import "gorm.io/gorm"

// PGCMedia 对齐 B 站 media 语义层：一个 media 下可包含多个 season（当前先 1:1 兼容演进）
type PGCMedia struct {
	gorm.Model
	MediaID uint64 `gorm:"column:media_id;comment:媒体ID;not null;uniqueIndex:uk_media_id" json:"media_id,string"`
	PGCType int    `gorm:"column:pgc_type;comment:PGC类型;not null;index:idx_media_type_status,priority:1"`
	Title   string `gorm:"type:varchar(255);comment:媒体标题;not null"`
	Cover   string `gorm:"type:varchar(255);comment:媒体封面;not null"`
	Desc    string `gorm:"type:varchar(1000);comment:媒体简介"`
	Year    int    `gorm:"comment:年份"`
	Area    string `gorm:"type:varchar(50);comment:地区"`
	Rating  float64 `gorm:"type:decimal(3,1);comment:评分"`
	Status  int    `gorm:"column:status;comment:审核状态;not null;index:idx_media_type_status,priority:2"`
}

func (table *PGCMedia) TableName() string {
	return "pgc_media"
}

