package routes

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/api/v1"
	"interastral-peace.com/alnitak/internal/middleware"
)

func CollectRelationRoutes(r *gin.RouterGroup) {
	relationGroup := r.Group("relation")

	relationAuth := relationGroup.Group("")
	relationAuth.Use(middleware.Auth())
	{
		// 关注
		relationAuth.POST("follow", api.Follow)
		// 取消关注
		relationAuth.POST("unfollow", api.Unfollow)
		// 获取用户关系
		relationAuth.GET("getUserRelation", api.GetUserRelation)
	}

	// 获取关注列表（可选认证，用于返回当前用户与列表用户的关系）
	relationGroup.GET("getFollowings", middleware.OptionalAuth(), api.GetFollowings)
	// 获取粉丝列表（可选认证）
	relationGroup.GET("getFollowers", middleware.OptionalAuth(), api.GetFollowers)
	// 获取关注和粉丝数
	relationGroup.GET("getFollowCount", api.GetFollowCount)
}
