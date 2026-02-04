package model

import "gorm.io/gorm"

type PGCContent struct {
	gorm.Model
	PGCID           uint    `gorm:"column:pgc_id;comment:PGC内容ID;not null;uniqueIndex:uk_pgc_id"`
	PGCType         int     `gorm:"column:pgc_type;comment:PGC类型;not null;index:idx_type_status,priority:1"`
	Title           string  `gorm:"type:varchar(255);comment:标题;not null"`
	Cover           string  `gorm:"type:varchar(255);comment:封面;not null"`
	Desc            string  `gorm:"type:varchar(1000);comment:简介"`
	Year            int     `gorm:"comment:年份"`
	Area            string  `gorm:"type:varchar(50);comment:地区"`
	Rating          float64 `gorm:"type:decimal(3,1);comment:评分"`
	IsOngoing       bool    `gorm:"column:is_ongoing;comment:是否连载中;default:0"`
	TotalEpisodes   int     `gorm:"column:total_episodes;comment:总集数;default:0"`
	CurrentEpisodes int     `gorm:"column:current_episodes;comment:已更新集数;default:0"`
	Status          int     `gorm:"column:status;comment:审核状态;not null;index:idx_type_status,priority:2"`
	OperatorID      uint    `gorm:"column:operator_id;comment:运营人员ID;index:idx_operator"`
}

func (table *PGCContent) TableName() string {
	return "pgc_content"
}
