package model

import "gorm.io/gorm"

type PGCEpisode struct {
	gorm.Model
	PGCID         uint64  `gorm:"column:pgc_id;comment:PGC内容ID;not null;uniqueIndex:uk_pgc_episode_no,priority:1" json:"pgc_id,string"`
	EpisodeNumber int     `gorm:"column:episode_number;comment:集数;not null;uniqueIndex:uk_pgc_episode_no,priority:2"`
	Title         string  `gorm:"type:varchar(255);comment:剧集标题"`
	VID           uint    `gorm:"column:vid;comment:关联视频ID;not null;index:idx_vid"`
	Duration      float64 `gorm:"comment:时长;default:0"`
	Status        int     `gorm:"column:status;comment:状态;not null;default:0"`
	PublishTime   string  `gorm:"column:publish_time;comment:发布时间"`
}

func (table *PGCEpisode) TableName() string {
	return "pgc_episode"
}
