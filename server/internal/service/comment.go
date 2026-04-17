package service

import (
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

func FindCommentById(id uint) (comment model.Comment, err error) {
	err = global.Mysql.First(&comment, id).Error
	return
}

func GetCommentInfo(commentId uint) (commentResp vo.CommentInfoResp) {
	comment, err := FindCommentById(commentId)
	if err != nil {
		return
	}
	commentResp = vo.CommentInfoResp{
		ID:        comment.ID,
		Content:   comment.Content,
		Type:      comment.Type,
		CreatedAt: comment.CreatedAt,
	}
	return
}

// CleanupCommentLikes 清理一组评论的点赞记录与点赞消息（用于删除评论时级联）
func CleanupCommentLikes(commentIds []uint) {
	if len(commentIds) == 0 {
		return
	}
	if err := global.Mysql.Where("comment_id IN ?", commentIds).Delete(&model.CommentLike{}).Error; err != nil {
		utils.ErrorLog("清理评论点赞记录失败", "comment_like", err.Error())
	}
	if err := global.Mysql.
		Where("comment_id IN ? AND `type` = ?", commentIds, global.CONTENT_TYPE_COMMENT).
		Delete(&model.LikeMessage{}).Error; err != nil {
		utils.ErrorLog("清理评论点赞消息失败", "msg_like", err.Error())
	}
}

// CleanupContentComments 删除内容（视频/文章）时级联清理所有评论相关数据
// contentType: global.CONTENT_TYPE_VIDEO 或 global.CONTENT_TYPE_ARTICLE
func CleanupContentComments(contentId uint, contentType int) {
	// 先把所有评论ID收集起来，用于清理点赞与点赞消息
	var commentIds []uint
	global.Mysql.Model(&model.Comment{}).
		Where("cid = ? AND `type` = ?", contentId, contentType).
		Pluck("id", &commentIds)

	// 删评论（软删除）
	if err := global.Mysql.
		Where("cid = ? AND `type` = ?", contentId, contentType).
		Delete(&model.Comment{}).Error; err != nil {
		utils.ErrorLog("删除内容关联评论失败", "comment", err.Error())
	}

	// 级联清理：comment_like + msg_like(type=3)
	CleanupCommentLikes(commentIds)

	// 清理内容自身的点赞消息（msg_like，type 对应内容类型）
	if err := global.Mysql.
		Where("cid = ? AND `type` = ?", contentId, contentType).
		Delete(&model.LikeMessage{}).Error; err != nil {
		utils.ErrorLog("清理内容点赞消息失败", "msg_like", err.Error())
	}

	// 清理回复消息
	if err := global.Mysql.
		Where("cid = ? AND `type` = ?", contentId, contentType).
		Delete(&model.ReplyMessage{}).Error; err != nil {
		utils.ErrorLog("清理回复消息失败", "msg_reply", err.Error())
	}
}
