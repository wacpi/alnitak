package utils

import (
	"regexp"
	"strings"
)

const MB = 1024 * 1024

// 是否不为空
func VerifyNotEmpty(val interface{}) bool {
	switch v := val.(type) {
	case string:
		return len(v) > 0
	case int:
		return v != 0
	case uint:
		return v != 0
	default:
		return false
	}
}

// 验证字符串长度
func VerifyStringLength(val, op string, length int) bool {
	switch op {
	case "<":
		return len(val) < length
	case ">":
		return len(val) > length
	case "=":
		return len(val) == length
	default:
		return false
	}
}

// 验证邮箱
func VerifyEmail(email string) bool {
	pattern := `\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*` //匹配电子邮箱
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(email)
}

// 验证是否为图片
func IsImgType(suffix string, allowedExts []string) bool {
	// 如果配置为空则使用默认值
	if len(allowedExts) == 0 {
		allowedExts = []string{"png", "jpeg", "jpg"}
	}

	// 标准化后缀格式（去除开头的点，转小写）
	suffix = strings.ToLower(strings.TrimPrefix(suffix, "."))

	// 检查后缀是否在允许列表中
	for _, ext := range allowedExts {
		if suffix == strings.ToLower(ext) {
			return true
		}
	}
	return false
}

// 验证是否为视频
func IsVideoType(suffix string, allowedExts []string) bool {
	// 如果配置为空则使用默认值
	if len(allowedExts) == 0 {
		allowedExts = []string{"mp4", "avi", "mkv", "mov", "flv", "wmv", "webm", "m4v", "mpeg", "mpg", "3gp"}
	}

	// 标准化后缀格式（去除开头的点，转小写）
	suffix = strings.ToLower(strings.TrimPrefix(suffix, "."))

	// 检查后缀是否在允许列表中
	for _, ext := range allowedExts {
		if suffix == strings.ToLower(ext) {
			return true
		}
	}
	return false
}

func FileSize(fileSize int64, count int64, targetSize int64) bool {
	if (fileSize * count) > (targetSize * MB) {
		return false
	}
	return true
}
