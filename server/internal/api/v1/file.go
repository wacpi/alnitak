package api

import (
	"fmt"
	"net/http"

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
	format := ctx.DefaultQuery("format", "m3u8") // m3u8 或 mpd

	file, err := service.GetVideoFile(ctx, resourceId, quality, format)
	if err != nil {
		resp.ForbiddenWithMessage(ctx, err.Error())
		return
	}

	if format == "mpd" {
		ctx.Writer.Header().Set("Content-type", "application/xml; charset=utf-8")
	} else {
		ctx.Writer.Header().Set("Content-type", "text/plain; charset=utf-8")
	}
	resp.OkWithString(ctx, file)
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

	if format == "mpd" {
		ctx.Writer.Header().Set("Content-type", "application/xml; charset=utf-8")
	} else {
		ctx.Writer.Header().Set("Content-type", "text/plain; charset=utf-8")
	}
	resp.OkWithString(ctx, file)
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

	// 使用oss（302临时重定向，避免浏览器缓存导致签名URL过期后请求失败）
	redirect := global.Storage.GetObjectUrl("video/" + dir + "/" + file)
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

		// 使用 http.ServeFile，它会自动处理：
		// - Range 请求（返回 206）
		// - Content-Type 检测
		// - 缓存头（Last-Modified, ETag）
		// - 高效的文件传输
		http.ServeFile(ctx.Writer, ctx.Request, filePath)
		return
	}

	// OSS 存储：重定向到 OSS URL
	redirect := global.Storage.GetObjectUrl("video/" + dir + "/" + file)
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

	// 不使用oss
	redirect := global.Storage.GetObjectUrl("image/" + file)

	// 开发模式下打印重定向信息
	if global.Config.Log.Mode == "dev" {
		fmt.Println("redirect", redirect, "image/"+file)
	}

	// 设置缓存头，告知浏览器缓存一天
	ctx.Header("Cache-Control", "public, max-age=86400, must-revalidate")
	ctx.Redirect(http.StatusMovedPermanently, redirect)
}
