package service

import "interastral-peace.com/alnitak/internal/global"

// getLocalBaseURL 在 Storage 为 local 且配置了 Domain 时返回该 Domain，用于构建“完整 URL”给前端。
// OSS/Domain 为空时返回空字符串，调用方自行拼接默认相对路径。
func getLocalBaseURL() string {
	if global.Config.Storage.OssType == "local" && global.Config.Storage.Domain != "" {
		return global.Config.Storage.Domain
	}
	return ""
}

