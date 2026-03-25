package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/internal/resp"
	"interastral-peace.com/alnitak/internal/service"
	jwt_parse "interastral-peace.com/alnitak/pkg/jwt"
	"interastral-peace.com/alnitak/utils"
)

const refreshCookieName = "refresh_token"

// 设置 refresh_token 的 HttpOnly Cookie（阶段 B：strict SSR 的基础）
func setRefreshCookie(ctx *gin.Context, refreshToken string) {
	// 兼容开发环境：本地 HTTP 不强制 Secure，线上 HTTPS 自动 Secure
	secure := ctx.Request.TLS != nil
	ctx.SetSameSite(http.SameSiteLaxMode)
	// 7 天（需与后端 refresh token 过期策略保持一致）
	maxAge := 7 * 24 * 60 * 60
	ctx.SetCookie(refreshCookieName, refreshToken, maxAge, "/", "", secure, true)
}

func clearRefreshCookie(ctx *gin.Context) {
	secure := ctx.Request.TLS != nil
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(refreshCookieName, "", -1, "/", "", secure, true)
}

// 注册
func Register(ctx *gin.Context) {
	// 获取参数
	var registerReq dto.RegisterReq
	if err := ctx.Bind(&registerReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	// 参数校验
	if !utils.VerifyEmail(registerReq.Email) {
		resp.FailWithMessage(ctx, "邮箱格式错误")
		return
	}

	if utils.VerifyStringLength(registerReq.Password, "<", 6) {
		resp.FailWithMessage(ctx, "密码长度不能小于6位")
		return
	}

	if !utils.VerifyStringLength(registerReq.Code, "=", 6) {
		resp.FailWithMessage(ctx, "验证码长度为6位")
		return
	}

	if err := service.UserRegister(ctx, registerReq); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回
	resp.Ok(ctx)
}

// 登录
func Login(ctx *gin.Context) {
	// 获取参数
	var loginReq dto.LoginReq
	if err := ctx.Bind(&loginReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	// 参数校验
	if !utils.VerifyEmail(loginReq.Email) {
		resp.FailWithMessage(ctx, "邮箱格式错误")
		return
	}

	if utils.VerifyStringLength(loginReq.Password, "<", 6) {
		resp.FailWithMessage(ctx, "密码长度不能小于6位")
		return
	}
	// 如果人机验证通过，删除登录尝试次数
	if utils.VerifyStringLength(loginReq.CaptchaId, ">", 0) {
		if cache.GetCaptchaStatus(loginReq.CaptchaId) == global.CAPTCHA_STATUS_PASS {
			// 删除登录尝试次数
			cache.DelLoginTryCount(loginReq.Email)
			// 删除人机验证状态
			cache.DelCaptchaStatus(loginReq.CaptchaId)
		}
	}

	// 读取登录尝试次数，超过3次进行滑块验证
	loginTryCount := cache.GetLoginTryCount(loginReq.Email)
	if loginTryCount >= 3 {
		captchaId := cache.CreateCaptchaStatus()

		resp.Result(ctx, -1, gin.H{"captchaId": captchaId}, "需要人机验证")
		return
	}

	accessToken, refreshToken, userId, err := service.UserLogin(ctx, loginReq)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 写入 HttpOnly Cookie，供 SSR/多标签页严格校验使用（兼容原 JSON 返回）
	setRefreshCookie(ctx, refreshToken)

	// 返回给前端
	resp.OkWithData(ctx, gin.H{"token": accessToken, "refreshToken": refreshToken, "userId": userId})
}

// 邮箱登录
func EmailLogin(ctx *gin.Context) {
	// 获取参数
	var loginReq dto.EmailLoginReq
	if err := ctx.Bind(&loginReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	// 参数校验
	if !utils.VerifyEmail(loginReq.Email) {
		resp.FailWithMessage(ctx, "邮箱格式错误")
		return
	}

	// 如果人机验证通过，删除登录尝试次数
	if utils.VerifyStringLength(loginReq.CaptchaId, ">", 0) {
		if cache.GetCaptchaStatus(loginReq.CaptchaId) == global.CAPTCHA_STATUS_PASS {
			// 删除登录尝试次数
			cache.DelLoginTryCount(loginReq.Email)
			// 删除人机验证状态
			cache.DelCaptchaStatus(loginReq.CaptchaId)
		}
	}

	// 读取登录尝试次数，超过3次进行滑块验证
	loginTryCount := cache.GetLoginTryCount(loginReq.Email)
	if loginTryCount >= 3 {
		captchaId := cache.CreateCaptchaStatus()

		resp.Result(ctx, -1, gin.H{"captchaId": captchaId}, "需要人机验证")
		return
	}

	accessToken, refreshToken, userId, err := service.EmailLogin(ctx, loginReq)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 写入 HttpOnly Cookie，供 SSR/多标签页严格校验使用（兼容原 JSON 返回）
	setRefreshCookie(ctx, refreshToken)

	// 返回给前端
	resp.OkWithData(ctx, gin.H{"token": accessToken, "refreshToken": refreshToken, "userId": userId})
}

// 刷新token
func UpdateToken(ctx *gin.Context) {
	var tokenReq dto.TokenReq
	// 兼容：既支持 body.refreshToken（旧前端），也支持从 HttpOnly Cookie 读取（阶段 B）
	_ = ctx.ShouldBindJSON(&tokenReq)
	if tokenReq.RefreshToken == "" {
		if v, err := ctx.Cookie(refreshCookieName); err == nil {
			tokenReq.RefreshToken = v
		}
	}
	if tokenReq.RefreshToken == "" {
		resp.Result(ctx, 2000, nil, "需要登录")
		return
	}

	accessToken, refreshToken, userId, err := service.UpdateToken(ctx, tokenReq)
	if err != nil {
		resp.Result(ctx, 2000, nil, err.Error())
		return
	}

	// refreshToken 可能被轮换，写回 cookie
	if refreshToken != "" {
		setRefreshCookie(ctx, refreshToken)
	}

	// 返回给前端
	resp.OkWithData(ctx, gin.H{"token": accessToken, "refreshToken": refreshToken, "userId": userId})
}

// 当前会话用户信息（支持 Authorization 或 refresh_token Cookie）
func Me(ctx *gin.Context) {
	// 1) 优先使用 Authorization access token（兼容旧模式）
	if tokenString := ctx.GetHeader("Authorization"); tokenString != "" {
		token, claims, err := jwt_parse.ParseToken(tokenString)
		if err == nil && token != nil && token.Valid && claims.TokenType == 0 {
			user := service.GetUserInfo(claims.UserId)
			resp.OkWithData(ctx, gin.H{"userInfo": user})
			return
		}
	}

	// 2) 使用 refresh_token Cookie 做严格校验（阶段 B）
	rt, err := ctx.Cookie(refreshCookieName)
	if err != nil || rt == "" {
		resp.Result(ctx, 2000, nil, "需要登录")
		return
	}

	accessToken, refreshToken, userId, err := service.UpdateToken(ctx, dto.TokenReq{RefreshToken: rt})
	if err != nil {
		clearRefreshCookie(ctx)
		resp.Result(ctx, 2000, nil, "需要登录")
		return
	}
	if refreshToken != "" {
		setRefreshCookie(ctx, refreshToken)
	}

	user := service.GetUserInfo(userId)
	// 保持兼容：仍返回 token 字段，便于老前端继续工作；阶段 B 前端可忽略
	resp.OkWithData(ctx, gin.H{"userInfo": user, "token": accessToken, "refreshToken": refreshToken, "userId": userId})
}

// 退出登录
func Logout(ctx *gin.Context) {
	var tokenReq dto.TokenReq
	_ = ctx.ShouldBindJSON(&tokenReq)
	if tokenReq.RefreshToken == "" {
		if v, err := ctx.Cookie(refreshCookieName); err == nil {
			tokenReq.RefreshToken = v
		}
	}

	service.Logout(ctx, tokenReq)
	clearRefreshCookie(ctx)

	// 返回给前端
	resp.Ok(ctx)
}

// 重置密码检查
func ResetPwdCheck(ctx *gin.Context) {
	// 获取参数
	var modifyPwdCheckReq dto.ModifyPwdCheckReq
	if err := ctx.Bind(&modifyPwdCheckReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	// 参数校验
	if !utils.VerifyEmail(modifyPwdCheckReq.Email) {
		resp.FailWithMessage(ctx, "邮箱格式错误")
		return
	}

	if utils.VerifyStringLength(modifyPwdCheckReq.CaptchaId, "=", 0) {
		captchaId := cache.CreateCaptchaStatus()
		resp.Result(ctx, -1, gin.H{"captchaId": captchaId}, "需要人机验证")
		return
	}

	switch cache.GetCaptchaStatus(modifyPwdCheckReq.CaptchaId) {
	case global.CAPTCHA_STATUS_ABSENT: // 如果未进行人机验证
		captchaId := cache.CreateCaptchaStatus()
		resp.Result(ctx, -1, gin.H{"captchaId": captchaId}, "需要人机验证")
		return
	case global.CAPTCHA_STATUS_PASS:
		cache.DelCaptchaStatus(modifyPwdCheckReq.CaptchaId) // 删除人机验证状态
	}

	if err := service.ResetPwdCheck(ctx, modifyPwdCheckReq.Email); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回给前端
	resp.Ok(ctx)
}

// 修改密码
func ModifyPwd(ctx *gin.Context) {
	var modifyPwdReq dto.ModifyPwdReq
	if err := ctx.Bind(&modifyPwdReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	// 参数校验
	if !utils.VerifyEmail(modifyPwdReq.Email) {
		resp.FailWithMessage(ctx, "邮箱格式错误")
		return
	}

	if utils.VerifyStringLength(modifyPwdReq.Password, "<", 6) {
		resp.FailWithMessage(ctx, "密码长度不能小于6位")
		return
	}

	if !utils.VerifyStringLength(modifyPwdReq.Code, "=", 6) {
		resp.FailWithMessage(ctx, "验证码长度为6位")
		return
	}

	if err := service.ModifyPwd(ctx, modifyPwdReq); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回给前端
	resp.Ok(ctx)
}
