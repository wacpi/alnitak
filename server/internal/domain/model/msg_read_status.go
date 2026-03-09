package model

import "gorm.io/gorm"

// MessageReadStatus 用户各消息分类的已读进度（公告/点赞/回复/@）
type MessageReadStatus struct {
	gorm.Model
	UserId     uint   `gorm:"uniqueIndex:idx_user_category;comment:用户ID;not null"`
	Category   string `gorm:"uniqueIndex:idx_user_category;size:20;comment:分类 announce/like/reply/at;not null"`
	ReadUpToId uint   `gorm:"comment:已读到的消息ID"`
}

func (MessageReadStatus) TableName() string {
	return "msg_read_status"
}
