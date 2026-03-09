package service

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/global"
)

const (
	CategoryAnnounce = "announce"
	CategoryLike     = "like"
	CategoryReply    = "reply"
	CategoryAt       = "at"
)

var validCategories = map[string]bool{
	CategoryAnnounce: true,
	CategoryLike:     true,
	CategoryReply:    true,
	CategoryAt:       true,
}

// GetReadStatus 获取当前用户各分类的已读进度，供客户端清数据后恢复
func GetReadStatus(ctx *gin.Context) map[string]uint {
	userId := ctx.GetUint("userId")
	out := map[string]uint{
		CategoryAnnounce: 0,
		CategoryLike:     0,
		CategoryReply:    0,
		CategoryAt:       0,
	}
	var list []model.MessageReadStatus
	if err := global.Mysql.Model(&model.MessageReadStatus{}).Where("user_id = ?", userId).Find(&list).Error; err != nil {
		return out
	}
	for _, r := range list {
		if _, ok := validCategories[r.Category]; ok && r.ReadUpToId > out[r.Category] {
			out[r.Category] = r.ReadUpToId
		}
	}
	return out
}

// SaveReadStatus 保存某分类的已读进度
func SaveReadStatus(ctx *gin.Context, category string, readUpToId uint) error {
	if !validCategories[category] || readUpToId == 0 {
		return nil
	}
	userId := ctx.GetUint("userId")
	var r model.MessageReadStatus
	err := global.Mysql.Unscoped().Where("user_id = ? and category = ?", userId, category).First(&r).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return global.Mysql.Create(&model.MessageReadStatus{
				UserId:     userId,
				Category:   category,
				ReadUpToId: readUpToId,
			}).Error
		}
		return err
	}
	if readUpToId > r.ReadUpToId {
		r.ReadUpToId = readUpToId
		r.DeletedAt = gorm.DeletedAt{} // 若之前被软删则恢复
		return global.Mysql.Unscoped().Save(&r).Error
	}
	return nil
}
