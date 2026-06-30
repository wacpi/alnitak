package routes

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/api/v1"
	"interastral-peace.com/alnitak/internal/middleware"
)

func CollectUploadRoutes(r *gin.RouterGroup) {

	uploadGroup := r.Group("upload")
	uploadGroup.Use(middleware.Auth())
	{
		// 图片上传（速率 + 10MB body 限制）
		uploadGroup.POST("image",
			middleware.MaxBodySize(middleware.UploadImgMaxBody),
			middleware.RateLimiter(middleware.UploadImgRateLimit),
			api.UploadImg)

		// 新建视频 / 新增分P（速率 + 1MB body 限制）
		uploadGroup.POST("video/:vid",
			middleware.MaxBodySize(middleware.UploadJSONMaxBody),
			middleware.RateLimiter(middleware.UploadCreateRateLimit),
			api.UploadVideoAdd)
		uploadGroup.POST("video",
			middleware.MaxBodySize(middleware.UploadJSONMaxBody),
			middleware.RateLimiter(middleware.UploadCreateRateLimit),
			api.UploadVideoCreate)

		// 查询分片（速率 + 1MB body 限制）
		uploadGroup.POST("checkVideo",
			middleware.MaxBodySize(middleware.UploadJSONMaxBody),
			middleware.RateLimiter(middleware.UploadCheckRateLimit),
			api.UploadVideoCheck)

		// 分片上传视频（速率 + 50MB body 限制）
		uploadGroup.POST("chunkVideo",
			middleware.MaxBodySize(middleware.UploadChunkMaxBody),
			middleware.RateLimiter(middleware.UploadChunkRateLimit),
			api.UploadVideoChunk)

		// 合并视频分片（速率 + 1MB body 限制）
		uploadGroup.POST("mergeVideo",
			middleware.MaxBodySize(middleware.UploadJSONMaxBody),
			middleware.RateLimiter(middleware.UploadChunkRateLimit),
			api.UploadVideoMerge)
	}
}
