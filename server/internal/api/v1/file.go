package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/internal/resp"
	"interastral-peace.com/alnitak/internal/service"
	"interastral-peace.com/alnitak/utils"
)

// 获取视频文件
func GetVideoFile(ctx *gin.Context) {
	quality := ctx.Query("quality")
	resourceId := utils.StringToUint(ctx.Query("resourceId"))
	format := ctx.DefaultQuery("format", "m3u8") // m3u8 / mpd / dash / m3u8video / m3u8audio

	file, err := service.GetVideoFile(ctx, resourceId, quality, format)
	if err != nil {
		resp.ForbiddenWithMessage(ctx, err.Error())
		return
	}

	// 关键：设置 Cache-Control 为 5 小时（与 OSS 签名过期时间一致）
	// 防止浏览器缓存 m3u8/mpd 播放列表，导致使用旧的签名 URL
	ctx.Header("Cache-Control", "public, max-age=18000, must-revalidate")
	ctx.Header("Pragma", "no-cache")

	switch format {
	case "mpd", "dash", "dash-unified":
		ctx.Data(http.StatusOK, "application/dash+xml; charset=utf-8", []byte(file))
	case "m3u8", "m3u8video", "m3u8audio":
		ctx.Data(http.StatusOK, "application/vnd.apple.mpegurl; charset=utf-8", []byte(file))
	default:
		ctx.Data(http.StatusOK, "application/json; charset=utf-8", []byte(file))
	}
}

// 获取视频文件(后台管理)
func GetVideoFileManage(ctx *gin.Context) {
	// 禁用缓存，确保审核时看到最新视频文件
	ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Expires", "0")

	quality := ctx.Query("quality")
	resourceId := utils.StringToUint(ctx.Query("resourceId"))
	format := ctx.DefaultQuery("format", "m3u8")

	file, err := service.GetVideoFileManage(ctx, resourceId, quality, format)
	if err != nil {
		resp.ForbiddenWithMessage(ctx, err.Error())
		return
	}

	switch format {
	case "mpd", "dash", "dash-unified":
		ctx.Data(http.StatusOK, "application/dash+xml; charset=utf-8", []byte(file))
	case "m3u8", "m3u8video", "m3u8audio":
		ctx.Data(http.StatusOK, "application/vnd.apple.mpegurl; charset=utf-8", []byte(file))
	default:
		ctx.Data(http.StatusOK, "application/json; charset=utf-8", []byte(file))
	}
}

// 获取视频切片（兼容模式：SegmentList）
func GetVideoSlice(ctx *gin.Context) {
	key := ctx.Query("key")
	file := ctx.Param("file")

	dir := service.GetVideoSliceDir(key)
	if dir == "" {
		resp.Forbidden(ctx)
		return
	}

	// 使用本地存储
	if global.Config.Storage.OssType == "local" {
		ctx.File("./upload/video/" + dir + "/" + file)
		return
	}

	// OSS 存储：重定向到公开URL
	redirect := global.GetOssUrl("video/" + dir + "/" + file)
	ctx.Redirect(http.StatusFound, redirect)
}

// GetVideoStream 获取视频流（B站风格：支持字节范围请求）
func GetVideoStream(ctx *gin.Context) {
	key := ctx.Query("key")
	file := ctx.Param("file")

	dir := service.GetVideoSliceDir(key)
	if dir == "" {
		resp.Forbidden(ctx)
		return
	}

	filePath := "./upload/video/" + dir + "/" + file

	// 使用本地存储时，支持 Range 请求
	if global.Config.Storage.OssType == "local" {
		// 设置 CORS 头
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Range")
		ctx.Header("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")

		// 处理 OPTIONS 预检请求
		if ctx.Request.Method == "OPTIONS" {
			ctx.Status(http.StatusOK)
			return
		}

		// .m4s 文件 Go 标准库不识别 MIME 类型，需要手动设置
		// 否则 iOS 原生 HLS 播放器可能无法识别
		if strings.HasSuffix(file, ".m4s") {
			ctx.Header("Content-Type", "video/iso.segment")
		}

		// 使用 http.ServeFile，它会自动处理：
		// - Range 请求（返回 206）
		// - Content-Type 检测（对于已知类型）
		// - 缓存头（Last-Modified, ETag）
		// - 高效的文件传输
		http.ServeFile(ctx.Writer, ctx.Request, filePath)
		return
	}

	// OSS 存储：重定向到公开URL
	redirect := global.GetOssUrl("video/" + dir + "/" + file)
	ctx.Redirect(http.StatusFound, redirect)
}

// 获取图片文件
func GetImgFile(ctx *gin.Context) {
	file := ctx.Param("file")

	// 使用本地存储
	if viper.GetString("storage.oss_type") == "local" {
		// 设置缓存头，告知浏览器缓存一天
		ctx.Header("Cache-Control", "public, max-age=86400, must-revalidate")
		ctx.File("./upload/image/" + file)
		return
	}

	// OSS 存储：重定向到公开URL
	redirect := global.GetOssUrl("image/" + file)

	// 开发模式下打印重定向信息
	if global.Config.Log.Mode == "dev" {
		fmt.Println("redirect", redirect, "image/"+file)
	}

	// 设置缓存头，告知浏览器缓存5小时（与OSS签名过期时间一致）
	ctx.Header("Cache-Control", "public, max-age=18000, must-revalidate")
	ctx.Redirect(http.StatusFound, redirect)
}
