package global

import (
	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
	"interastral-peace.com/alnitak/internal/config"
	"interastral-peace.com/alnitak/pkg/casbin"
	"interastral-peace.com/alnitak/pkg/oss"
	"interastral-peace.com/alnitak/pkg/redis"
)

var (
	Config            *config.Config
	Mysql             *gorm.DB
	Redis             *redis.Redis
	Casbin            *casbin.Casbin
	Storage           oss.Storage
	SnowflakeNode     *snowflake.Node
	VideoPartitionMap map[uint]uint
)

// GetOssUrl 获取 OSS 访问 URL
// 公开bucket: 直接拼接公开URL（短、不过期）
// 私有bucket: 通过SDK生成预签名URL（带认证参数）
func GetOssUrl(objectKey string) string {
	if Config.Storage.Private {
		return Storage.GetObjectUrl(objectKey)
	}
	scheme := "http://"
	if Config.Storage.UseSSL {
		scheme = "https://"
	}
	domain := Config.Storage.Domain
	if domain == "" {
		domain = Config.Storage.Endpoint
	}
	return scheme + domain + "/" + Config.Storage.Bucket + "/" + objectKey
}
