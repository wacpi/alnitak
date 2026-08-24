package oss

import (
	"errors"
	"io"

	"interastral-peace.com/alnitak/internal/config"
	"interastral-peace.com/alnitak/utils"
)

const (
	ALIYUN  = "aliyun"
	MINIO   = "minio"
	TENCENT = "tencent"
	//QINIU    = "qiniu"
	R2TORAGE = "cloudflare"
)

type Storage interface {
	GetObjectToFile(objectKey, downloadedFileName string) error
	DeleteObject(objectKey string) error
	PutObject(objectKey string, reader io.Reader) error
	PutObjectFromFile(objectKey, filePath string) error
	IsExists(objectKey string) (bool, error)
	GetObjectUrl(objectKey string) string
	GetObjectReader(objectKey string) (io.ReadCloser, error)
}

func InitStorage(c config.Storage) Storage {
	clientConfig := buildOssConfig(c)

	s, err := initOss(c.OssType, clientConfig)
	if err != nil {
		utils.ErrorLog("oss初始化失败", "oss", err.Error())
		panic(err)
	}

	return s
}

// InitBackupStorage 初始化备用 OSS 实例（多源容灾），忽略 OssType=local 或配置为空。
// 继承主配置的 Private/UseSSL/UploadTimeout。
func InitBackupStorage(primary config.Storage) Storage {
	if primary.Backup == nil || primary.Backup.OssType == "" || primary.Backup.OssType == "local" {
		return nil
	}

	merged := primary.Backup.ToStorageConfig(primary)
	return InitStorage(merged)
}

func buildOssConfig(c config.Storage) Config {
	return Config{
		KeyID:     c.KeyId,
		KeySecret: c.KeySecret,
		Bucket:    c.Bucket,
		Endpoint:  c.Endpoint,
		AppID:     c.AppId,
		Region:    c.Region,
		Domain:    c.Domain,
		Private:   c.Private,
		UseSSL:    c.UseSSL,
		Timeout:   c.UploadTimeout,
	}
}

func initOss(ossName string, config Config) (Storage, error) {
	switch ossName {
	case ALIYUN:
		return newAliyun(config)
	case MINIO:
		return newMinio(config)
	case TENCENT:
		return newTencentCOS(config)
	//case QINIU:
	//	return newQiniuOSS(config)
	case R2TORAGE:
		return newR2Storage(config)
	default:
		return nil, errors.New("driver not exists")
	}
}
