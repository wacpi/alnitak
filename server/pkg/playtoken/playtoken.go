package playtoken

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"interastral-peace.com/alnitak/internal/global"
)

const issuerPlay = "alnitak-play"
const issuerStream = "alnitak-stream"

// PlayGrantClaims 播放授权：换取音视频分片 URL 前必须先持有有效 grant。
type PlayGrantClaims struct {
	UserID            uint   `json:"uid"`
	ResourceShortID   string `json:"rsid"`
	VideoID           uint   `json:"vid"`
	jwt.RegisteredClaims
}

// StreamClaims 单个媒体文件（m4s 等）访问授权，嵌入在 /video/stream 的 st 参数。
type StreamClaims struct {
	Dir  string `json:"dir"`
	File string `json:"file"`
	jwt.RegisteredClaims
}

func playSecret() []byte {
	s := global.Config.Security.PlayJwtSecret
	if s == "" {
		s = global.Config.Security.AccessJwtSecret + "_play_grant"
	}
	return []byte(s)
}

func streamSecret() []byte {
	s := global.Config.Security.StreamJwtSecret
	if s == "" {
		s = global.Config.Security.AccessJwtSecret + "_stream_slice"
	}
	return []byte(s)
}

// IssuePlayGrant 签发播放授权 JWT（建议 TTL 2~10 分钟，前端在过期前静默换发）。
func IssuePlayGrant(userID, videoID uint, resourceShortID string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := &PlayGrantClaims{
		UserID:          userID,
		ResourceShortID: resourceShortID,
		VideoID:         videoID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    issuerPlay,
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(playSecret())
}

// ParsePlayGrant 校验播放授权。
func ParsePlayGrant(tokenStr string) (*PlayGrantClaims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &PlayGrantClaims{}, func(t *jwt.Token) (interface{}, error) {
		return playSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*PlayGrantClaims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid play grant")
	}
	return claims, nil
}

// IssueStreamToken 为具体分片路径签发短期 JWT。
func IssueStreamToken(dir, file string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := &StreamClaims{
		Dir:  dir,
		File: file,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    issuerStream,
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(streamSecret())
}

// ParseStreamToken 校验分片访问 token。
func ParseStreamToken(tokenStr string) (*StreamClaims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &StreamClaims{}, func(t *jwt.Token) (interface{}, error) {
		return streamSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*StreamClaims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid stream token")
	}
	return claims, nil
}
