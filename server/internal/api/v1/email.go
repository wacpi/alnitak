package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/internal/resp"
	"interastral-peace.com/alnitak/pkg/mail"
	"interastral-peace.com/alnitak/utils"
)

func SendRegisterEmailCode(ctx *gin.Context) {
	// 获取参数
	var sendEmailReq dto.SendEmailReq
	if err := ctx.Bind(&sendEmailReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	// 参数校验
	if !utils.VerifyEmail(sendEmailReq.Email) {
		resp.FailWithMessage(ctx, "邮箱格式错误")
		return
	}

	// 检查发送冷却
	if cooldown := cache.GetEmailCodeCooldown(sendEmailReq.Email); cooldown > 0 {
		resp.FailWithDetailed(ctx, gin.H{"countdown": cooldown}, "发送过于频繁，请稍后再试")
		return
	}

	if utils.VerifyStringLength(sendEmailReq.CaptchaId, "=", 0) {
		captchaId := cache.CreateCaptchaStatus()
		resp.Result(ctx, -1, gin.H{"captchaId": captchaId}, "需要人机验证")
		return
	}

	// 如果未进行人机验证
	if cache.GetCaptchaStatus(sendEmailReq.CaptchaId) == global.CAPTCHA_STATUS_ABSENT {
		captchaId := cache.CreateCaptchaStatus()
		resp.Result(ctx, -1, gin.H{"captchaId": captchaId}, "需要人机验证")
		return
	}

	// 生成code
	code := utils.GenerateNumberCode(6)

	// 发送code
	if global.Config.Mail.Debug {
		// 验证码debug模式不发送邮件
		zap.L().Debug("邮箱:" + sendEmailReq.Email + ",验证码:" + code)
	} else {
		if err := mail.SendCaptcha(sendEmailReq.Email, code); err != nil {
			utils.ErrorLog("邮箱验证码发送失败", "email", err.Error())
			resp.FailWithMessage(ctx, "邮箱验证码发送失败，请检查邮箱是否正确")
			return
		}
	}

	// code放入缓存
	cache.SetEmailCode(sendEmailReq.Email, code)
	// 设置发送冷却
	cache.SetEmailCodeCooldown(sendEmailReq.Email)
	// 删除人机验证状态
	cache.DelCaptchaStatus(sendEmailReq.CaptchaId)

	resp.OkWithDetailed(ctx, gin.H{"countdown": 60}, "验证码已发送，请查收邮箱")
}
