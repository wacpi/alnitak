package routes

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/api/v1"
)

func CollectAuthRoutes(r *gin.RouterGroup) {

	authGroup := r.Group("auth")

	// 用户注册
	authGroup.POST("register", api.Register)
	// 用户登录(密码)
	authGroup.POST("login", api.Login)
	// 用户登录(邮箱)
	authGroup.POST("login/email", api.EmailLogin)
	// 当前会话用户（支持 Authorization / Cookie）
	authGroup.GET("me", api.Me)
	// 退出登录：凭 body 或 HttpOnly refresh_token Cookie，无需 Authorization（行业常见做法）
	authGroup.POST("logout", api.Logout)
	// 更新token
	authGroup.POST("updateToken", api.UpdateToken)
	// 修改密码检查
	authGroup.POST("resetpwdCheck", api.ResetPwdCheck)
	// 修改密码
	authGroup.POST("modifyPwd", api.ModifyPwd)
}
