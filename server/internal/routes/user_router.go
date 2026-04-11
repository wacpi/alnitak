package routes

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/api/v1"
	"interastral-peace.com/alnitak/internal/middleware"
)

func CollectUserRoutes(r *gin.RouterGroup) {
	userGroup := r.Group("user")
	// 获取用户基本信息
	userGroup.GET("getUserBaseInfo", api.GetUserBaseInfo)
	// 搜索用户 / UP
	userGroup.POST("searchUser", api.SearchUser)

	userAuth := userGroup.Use(middleware.Auth())
	{
		// 用户获取个人信息
		userAuth.GET("getUserInfo", api.GetUserInfo)
		// 用户编辑个人信息
		userAuth.PUT("editUserInfo", api.EditUserInfo)
		// 获取用户列表（后台管理）
		userAuth.POST("getUserListManage", api.GetUserListManage)
		// 编辑用户信息（后台管理）
		userAuth.PUT("editUserInfoManage", api.EditUserInfoManage)
		// 设置用户角色
		userAuth.PUT("editUserRole", api.EditUserRole)
		// 封禁用户（后台管理）
		userAuth.PUT("banUser", api.BanUser)
		// 解封用户（后台管理）
		userAuth.PUT("unBanUser", api.UnBanUser)
		// 获取封禁记录（后台管理）
		userAuth.GET("getUserBanRecord", api.GetUserBanRecord)
		// 删除用户
		userAuth.DELETE("deleteUser/:id", api.DeleteUser)
	}
}
