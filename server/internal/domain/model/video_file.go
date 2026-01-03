package model

import "gorm.io/gorm"

type VideoFile struct {
	gorm.Model
	Uid          uint   `gorm:"comment:用户ID;not null;index:idx_uid_hash,priority:1"`
	OriginalName string `gorm:"type:varchar(500);comment:原始文件名;"` // 扩大长度支持长文件名
	DirName      string `gorm:"type:varchar(20);comment:目录名称;index"`
	Hash         string `gorm:"type:varchar(64);comment:文件hash;index:idx_uid_hash,priority:2,unique"`
	ChunksCount  int    `gorm:"comment:分片数量;"`
}

func (table *VideoFile) TableName() string {
	return "video_file"
}
