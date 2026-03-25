package cache

import (
	"interastral-peace.com/alnitak/internal/global"
)

// 获取邮箱验证码
func GetEmailCode(email string) string {
	return global.Redis.Get(EMAIL_CODE_KEY + email)
}

// 保存邮箱验证码
func SetEmailCode(email string, code string) {
	global.Redis.Set(EMAIL_CODE_KEY+email, code, EMIAL_CODE_EXPIRATION_TIME)
}

// 删除邮箱验证码
func DelEmailCode(email string) {
	global.Redis.Del(EMAIL_CODE_KEY + email)
}

// 获取邮箱验证码发送冷却剩余时间（秒），0表示无冷却
func GetEmailCodeCooldown(email string) int {
	ttl := global.Redis.TTL(EMAIL_CODE_COOLDOWN_KEY + email)
	if ttl > 0 {
		return int(ttl.Seconds())
	}
	return 0
}

// 设置邮箱验证码发送冷却
func SetEmailCodeCooldown(email string) {
	global.Redis.Set(EMAIL_CODE_COOLDOWN_KEY+email, "1", EMAIL_CODE_COOLDOWN_TIME)
}
