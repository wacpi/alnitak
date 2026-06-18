package cache

import (
	"time"

	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

func IsRefreshTokenExist(userId uint, token string) bool {
	return global.Redis.ZScore(REFRESH_TOKEN_KEY+utils.UintToString(userId), token) != 0
}

func SetRefreshToken(id uint, token string) {
	key := REFRESH_TOKEN_KEY + utils.UintToString(id)
	if global.Redis.ZCard(key) >= MAX_LOGIN_LIMIT {
		// 移除最旧的，保留最近 MAX_LOGIN_LIMIT - 1 个（加上即将添加的新 token 共 MAX_LOGIN_LIMIT 个）
		global.Redis.ZRemRangeByRank(key, 0, -MAX_LOGIN_LIMIT)
	}

	global.Redis.ZAdd(key, float64(time.Now().Add(REFRESH_TOKEN_EXPIRATION_TIME).Unix()), token)
}

func DelRefreshToken(id uint, token string) {
	global.Redis.ZRem(REFRESH_TOKEN_KEY+utils.UintToString(id), token)
}

// DelAllRefreshToken 删除用户的所有 refreshToken（改密码/强制下线时使用）
func DelAllRefreshToken(id uint) {
	global.Redis.Del(REFRESH_TOKEN_KEY + utils.UintToString(id))
}
