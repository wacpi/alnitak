package routes

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/api/v1"
	"interastral-peace.com/alnitak/internal/middleware"
)

func CollectUserAuthRoutes(r *gin.RouterGroup) {
	authGroup := r.Group("auth")

	// 公开接口
	// 获取认证类型列表（公开）
	authGroup.GET("type/list", api.GetAuthTypeList)
	// 获取用户认证列表（公开）
	authGroup.GET("user/list", api.GetUserAuthList)
	// 获取用户主要认证（公开）
	authGroup.GET("user/primary", api.GetUserPrimaryAuth)
	// 获取指定用户的认证信息
	authGroup.GET("user/:uid/auth", api.GetUserAuthByUid)

	// 管理接口（需要登录）
	authManage := authGroup.Group("")
	authManage.Use(middleware.Auth())
	{
		// 认证类型管理
		authManage.POST("type/add", api.AddAuthType)
		authManage.PUT("type/edit", api.EditAuthType)
		authManage.DELETE("type/:id", api.DeleteAuthType)
		authManage.GET("type/all", api.GetAllAuthTypeList)
		authManage.GET("type/:id", api.GetAuthTypeByID)

		// 用户认证管理
		authManage.POST("user/add", api.AddUserAuth)
		authManage.PUT("user/edit", api.EditUserAuth)
		authManage.DELETE("user", api.DeleteUserAuth)
		authManage.GET("user/all", api.GetUserAuthListWithUser)
		authManage.GET("user/:id", api.GetUserAuthByID)
	}
}