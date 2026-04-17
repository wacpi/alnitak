package routes

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/api/v1"
	"interastral-peace.com/alnitak/internal/middleware"
)

func CollectCommentRoutes(r *gin.RouterGroup) {
	commentGroup := r.Group("comment")

	commentAuth := commentGroup.Group("")
	commentAuth.Use(middleware.Auth())
	{
		// 发表评论或回复
		commentAuth.POST("video/addComment", middleware.Ban(), api.AddVideoComment)
		// 删除评论或回复
		commentAuth.DELETE("video/deleteComment/:id", api.DeleteVideoComment)
		// 获取评论列表
		commentAuth.GET("video/getCommentList", api.GetVideoCommentList)
		// 点赞评论
		commentAuth.POST("video/like/:id", api.LikeComment)
		// 取消点赞评论
		commentAuth.DELETE("video/like/:id", api.UnlikeComment)
		// 获取评论点赞状态
		commentAuth.GET("video/getLikeStatus", api.GetCommentLikeStatus)
		// 发表文章评论或回复
		commentAuth.POST("article/addComment", middleware.Ban(), api.AddArticleComment)
		// 删除评论或回复
		commentAuth.DELETE("article/deleteComment/:id", api.DeleteArticleComment)
		// 获取评论列表
		commentAuth.GET("article/getCommentList", api.GetArticleCommentList)
		// 点赞评论（文章）
		commentAuth.POST("article/like/:id", api.LikeComment)
		// 取消点赞评论（文章）
		commentAuth.DELETE("article/like/:id", api.UnlikeComment)
		// 获取评论点赞状态（文章）
		commentAuth.GET("article/getLikeStatus", api.GetCommentLikeStatus)
	}

	// 获取视频评论
	commentGroup.GET("video/getComment", api.GetVideoComment)
	// 获取视频回复
	commentGroup.GET("video/getReply", api.GetVideoReply)
	// 获取文章评论
	commentGroup.GET("article/getComment", api.GetArticleComment)
	// 获取文章回复
	commentGroup.GET("article/getReply", api.GetArticleReply)
}
