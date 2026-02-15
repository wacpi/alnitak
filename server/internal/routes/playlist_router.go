package routes

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/api/v1"
	"interastral-peace.com/alnitak/internal/middleware"
)

func CollectPlaylistRoutes(r *gin.RouterGroup) {
	playlistGroup := r.Group("playlist")

	// 需要登录的接口
	playlistAuth := playlistGroup.Group("")
	playlistAuth.Use(middleware.Auth())
	{
		// 创建合集
		playlistAuth.POST("add", api.AddPlaylist)
		// 编辑合集
		playlistAuth.PUT("edit", api.EditPlaylist)
		// 删除合集
		playlistAuth.DELETE("del/:id", api.DeletePlaylist)
		// 获取自己的合集列表
		playlistAuth.GET("myList", api.GetMyPlaylist)

		// 获取用户所有合集中的视频ID映射
		playlistAuth.GET("video/myVideoIds", api.GetMyPlaylistVideoIds)
		// 添加视频到合集
		playlistAuth.POST("video/add", api.AddPlaylistVideo)
		// 从合集移除视频
		playlistAuth.POST("video/del", api.DelPlaylistVideo)
		// 调整合集视频排序
		playlistAuth.POST("video/sort", api.SortPlaylistVideo)

		// 合集审核（后台管理）
		playlistAuth.POST("getReviewPlaylistList", api.GetReviewPlaylistList)
		playlistAuth.POST("reviewApproved", api.ReviewPlaylistApproved)
		playlistAuth.POST("reviewFailed", api.ReviewPlaylistFailed)
		// 获取合集审核记录
		playlistAuth.GET("getPlaylistReviewRecord", api.GetPlaylistReviewRecord)

		// 合集管理（后台管理）
		playlistAuth.POST("getPlaylistListManage", api.GetPlaylistListManage)
		playlistAuth.DELETE("deletePlaylistManage/:id", api.DeletePlaylistManage)
	}

	// 公开接口（无需登录）
	// 获取合集详情
	playlistGroup.GET("info", api.GetPlaylistInfo)
	// 获取合集视频列表
	playlistGroup.GET("video/list", api.GetPlaylistVideoList)
	// 获取用户的公开合集列表
	playlistGroup.GET("userList", api.GetUserPlaylist)

	// 视频合集相关接口（无需登录）
	// 获取视频所属的合集列表
	playlistGroup.GET("video/playlists", api.GetVideoPlaylists)
	// 获取视频的主要合集
	playlistGroup.GET("video/primaryPlaylist", api.GetVideoPrimaryPlaylist)
	// 获取合集视频列表（包含多分P展开）
	playlistGroup.GET("video/listWithParts", api.GetPlaylistVideoListWithParts)
}
