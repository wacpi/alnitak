package model

import "gorm.io/gorm"

// VideoFileStatus 视频文件状态
const (
	FileStatusUploading   = 0 // 上传中
	FileStatusMerged      = 1 // 已合并，待转码
	FileStatusTranscoding = 2 // 转码中
	FileStatusReady       = 3 // 已就绪（转码完成）
)

// VideoFile 全局视频文件表（按hash唯一，支持多用户共享）
type VideoFile struct {
	gorm.Model
	Hash         string `gorm:"type:varchar(64);comment:文件hash;uniqueIndex"`
	OriginalName string `gorm:"type:varchar(255);comment:原始文件名;"`
	DirName      string `gorm:"type:varchar(20);comment:目录名称;uniqueIndex"`
	ChunksCount  int    `gorm:"comment:分片数量;"`
	Status       int    `gorm:"comment:文件状态(0上传中/1已合并/2转码中/3已就绪);default:0;index"`
	RefCount     int    `gorm:"comment:引用计数;default:0"`
	UploaderUid  uint   `gorm:"column:uid;comment:首次上传者ID;index"`
}

func (table *VideoFile) TableName() string {
	return "video_file"
}

// VideoFileRef 用户-文件引用关系表
type VideoFileRef struct {
	gorm.Model
	Uid        uint `gorm:"comment:用户ID;not null;index:idx_uid_file,priority:1"`
	FileID     uint `gorm:"comment:文件ID;not null;index:idx_uid_file,priority:2"`
	ResourceID uint `gorm:"comment:关联的资源ID;index"`
}

func (table *VideoFileRef) TableName() string {
	return "video_file_ref"
}
