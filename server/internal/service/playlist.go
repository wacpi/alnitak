package service

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

const (
	maxPlaylistPerUser = 50  // 每用户最多创建合集数
	maxVideoPerPlaylist = 200 // 每合集最多视频数
	sortStep           = 1000 // 排序步长
)

// AddPlaylist 创建合集
func AddPlaylist(ctx *gin.Context, req dto.AddPlaylistReq) (uint, error) {
	userId := ctx.GetUint("userId")

	if utils.VerifyStringLength(req.Title, ">", 100) {
		return 0, errors.New("合集标题不能超过100个字符")
	}

	// 检查用户合集数量限制
	var count int64
	global.Mysql.Model(&model.Playlist{}).Where("uid = ?", userId).Count(&count)
	if count >= maxPlaylistPerUser {
		return 0, fmt.Errorf("最多创建%d个合集", maxPlaylistPerUser)
	}

	playlist := model.Playlist{
		Uid:    userId,
		Title:  req.Title,
		Cover:  req.Cover,
		Desc:   req.Desc,
		Status: global.WAITING_REVIEW,
	}
	if err := global.Mysql.Create(&playlist).Error; err != nil {
		utils.ErrorLog("创建合集失败", "playlist", err.Error())
		return 0, errors.New("创建合集失败")
	}

	return playlist.ID, nil
}

// EditPlaylist 编辑合集
func EditPlaylist(ctx *gin.Context, req dto.EditPlaylistReq) error {
	userId := ctx.GetUint("userId")

	var playlist model.Playlist
	if err := global.Mysql.First(&playlist, req.ID).Error; err != nil {
		return errors.New("合集不存在")
	}

	if playlist.Uid != userId {
		return errors.New("无权操作")
	}

	if utils.VerifyStringLength(req.Title, ">", 100) {
		return errors.New("合集标题不能超过100个字符")
	}

	if err := global.Mysql.Model(&model.Playlist{}).Where("id = ?", req.ID).Updates(map[string]interface{}{
		"title":   req.Title,
		"cover":   req.Cover,
		"desc":    req.Desc,
		"is_open": req.IsOpen,
		"status":  global.WAITING_REVIEW,
	}).Error; err != nil {
		utils.ErrorLog("编辑合集失败", "playlist", err.Error())
		return errors.New("编辑失败")
	}

	return nil
}

// DeletePlaylist 删除合集
func DeletePlaylist(ctx *gin.Context, id uint) error {
	userId := ctx.GetUint("userId")

	var playlist model.Playlist
	if err := global.Mysql.First(&playlist, id).Error; err != nil {
		return errors.New("合集不存在")
	}

	if playlist.Uid != userId {
		return errors.New("无权操作")
	}

	// 删除关联视频
	global.Mysql.Where("playlist_id = ?", id).Delete(&model.PlaylistVideo{})
	// 删除合集
	if err := global.Mysql.Where("id = ?", id).Delete(&model.Playlist{}).Error; err != nil {
		utils.ErrorLog("删除合集失败", "playlist", err.Error())
		return errors.New("删除失败")
	}

	return nil
}

// GetMyPlaylist 获取自己的合集列表
func GetMyPlaylist(ctx *gin.Context) (total int64, list []vo.PlaylistListResp) {
	userId := ctx.GetUint("userId")
	global.Mysql.Model(&model.Playlist{}).Where("uid = ?", userId).Count(&total)
	global.Mysql.Model(&model.Playlist{}).Select(vo.PLAYLIST_LIST_FIELD).
		Where("uid = ?", userId).Order("created_at desc").Scan(&list)
	return
}

// GetPlaylistInfo 获取合集详情
func GetPlaylistInfo(ctx *gin.Context, id uint) (result vo.PlaylistResp, err error) {
	if err = global.Mysql.Model(&model.Playlist{}).Select(vo.PLAYLIST_FIELD).
		Where("id = ?", id).Scan(&result).Error; err != nil {
		return result, errors.New("合集不存在")
	}

	if result.ID == 0 {
		return result, errors.New("合集不存在")
	}

	// 非公开或未审核通过的合集只有创建者可见
	userId := ctx.GetUint("userId")
	if result.Uid != userId && (!result.IsOpen || result.Status != global.AUDIT_APPROVED) {
		return result, errors.New("合集不存在")
	}

	result.Author = GetUserBaseInfo(result.Uid)

	// 增加浏览量
	global.Mysql.Model(&model.Playlist{}).Where("id = ?", id).
		UpdateColumn("views", result.Views+1)

	return
}

// GetUserPlaylist 获取用户的公开合集列表
func GetUserPlaylist(ctx *gin.Context, uid uint, page, pageSize int) (total int64, list []vo.PlaylistListResp) {
	global.Mysql.Model(&model.Playlist{}).Where("uid = ? AND is_open = ? AND status = ?", uid, true, global.AUDIT_APPROVED).Count(&total)
	global.Mysql.Model(&model.Playlist{}).Select(vo.PLAYLIST_LIST_FIELD).
		Where("uid = ? AND is_open = ? AND status = ?", uid, true, global.AUDIT_APPROVED).
		Order("created_at desc").
		Limit(pageSize).Offset((page - 1) * pageSize).Scan(&list)
	return
}

// AddPlaylistVideo 添加视频到合集
func AddPlaylistVideo(ctx *gin.Context, req dto.PlaylistVideoAddReq) error {
	userId := ctx.GetUint("userId")

	var playlist model.Playlist
	if err := global.Mysql.First(&playlist, req.PlaylistID).Error; err != nil {
		return errors.New("合集不存在")
	}

	if playlist.Uid != userId {
		return errors.New("无权操作")
	}

	// 检查视频数量限制
	if playlist.VideoCount+len(req.Vids) > maxVideoPerPlaylist {
		return fmt.Errorf("合集视频数量不能超过%d", maxVideoPerPlaylist)
	}

	// 获取当前最大排序值
	var maxSort int64
	global.Mysql.Model(&model.PlaylistVideo{}).Where("playlist_id = ?", req.PlaylistID).
		Select("COALESCE(MAX(sort), 0)").Scan(&maxSort)

	addCount := 0
	for _, vid := range req.Vids {
		// 校验视频存在且审核通过
		var video model.Video
		if err := global.Mysql.Where("id = ? AND uid = ? AND status = ?", vid, userId, global.AUDIT_APPROVED).
			First(&video).Error; err != nil {
			continue // 跳过不符合条件的视频
		}

		// 检查是否已在合集中
		var exists int64
		global.Mysql.Model(&model.PlaylistVideo{}).
			Where("playlist_id = ? AND vid = ?", req.PlaylistID, vid).Count(&exists)
		if exists > 0 {
			continue
		}

		maxSort += sortStep
		global.Mysql.Create(&model.PlaylistVideo{
			PlaylistID: req.PlaylistID,
			Vid:        vid,
			Sort:       maxSort,
		})
		addCount++
	}

	if addCount > 0 {
		// 更新视频数量
		global.Mysql.Model(&model.Playlist{}).Where("id = ?", req.PlaylistID).
			UpdateColumn("video_count", playlist.VideoCount+addCount)
	}

	return nil
}

// DelPlaylistVideo 从合集移除视频
func DelPlaylistVideo(ctx *gin.Context, req dto.PlaylistVideoDelReq) error {
	userId := ctx.GetUint("userId")

	var playlist model.Playlist
	if err := global.Mysql.First(&playlist, req.PlaylistID).Error; err != nil {
		return errors.New("合集不存在")
	}

	if playlist.Uid != userId {
		return errors.New("无权操作")
	}

	result := global.Mysql.Where("playlist_id = ? AND vid IN ?", req.PlaylistID, req.Vids).
		Delete(&model.PlaylistVideo{})

	if result.RowsAffected > 0 {
		// 更新视频数量
		var count int64
		global.Mysql.Model(&model.PlaylistVideo{}).Where("playlist_id = ?", req.PlaylistID).Count(&count)
		global.Mysql.Model(&model.Playlist{}).Where("id = ?", req.PlaylistID).
			Update("video_count", count)
	}

	return nil
}

// SortPlaylistVideo 调整合集视频排序（前端传入完整的视频ID顺序数组）
func SortPlaylistVideo(ctx *gin.Context, req dto.PlaylistVideoSortReq) error {
	userId := ctx.GetUint("userId")

	var playlist model.Playlist
	if err := global.Mysql.First(&playlist, req.PlaylistID).Error; err != nil {
		return errors.New("合集不存在")
	}

	if playlist.Uid != userId {
		return errors.New("无权操作")
	}

	// 按传入的顺序重新分配排序值
	for i, vid := range req.Vids {
		newSort := int64((i + 1) * sortStep)
		global.Mysql.Model(&model.PlaylistVideo{}).
			Where("playlist_id = ? AND vid = ?", req.PlaylistID, vid).
			Update("sort", newSort)
	}

	return nil
}

// GetPlaylistListManage 获取全站合集列表（后台管理）
func GetPlaylistListManage(page, pageSize int) (total int64, list []vo.PlaylistResp) {
	global.Mysql.Model(&model.Playlist{}).Count(&total)
	global.Mysql.Model(&model.Playlist{}).Select(vo.PLAYLIST_FIELD).
		Order("created_at desc").
		Limit(pageSize).Offset((page - 1) * pageSize).Scan(&list)

	for i := 0; i < len(list); i++ {
		list[i].Author = GetUserBaseInfo(list[i].Uid)
	}

	return
}

// DeletePlaylistManage 删除合集（后台管理，无需校验所有者）
func DeletePlaylistManage(id uint) error {
	var playlist model.Playlist
	if err := global.Mysql.First(&playlist, id).Error; err != nil {
		return errors.New("合集不存在")
	}

	// 删除关联视频
	global.Mysql.Where("playlist_id = ?", id).Delete(&model.PlaylistVideo{})
	// 删除合集
	if err := global.Mysql.Where("id = ?", id).Delete(&model.Playlist{}).Error; err != nil {
		utils.ErrorLog("删除合集失败", "playlist", err.Error())
		return errors.New("删除失败")
	}

	return nil
}

// GetPlaylistVideoList 获取合集视频列表
func GetPlaylistVideoList(ctx *gin.Context, playlistID uint, page, pageSize int) (total int64, list []vo.PlaylistVideoResp) {
	// 检查合集是否存在且可见
	var playlist model.Playlist
	if err := global.Mysql.First(&playlist, playlistID).Error; err != nil {
		return 0, nil
	}

	userId := ctx.GetUint("userId")
	if !playlist.IsOpen && playlist.Uid != userId {
		return 0, nil
	}

	global.Mysql.Model(&model.PlaylistVideo{}).Where("playlist_id = ?", playlistID).Count(&total)

	global.Mysql.Table("playlist_video").
		Select("video.id as vid, video.title, video.cover, video.duration, video.clicks, video.desc, video.created_at").
		Joins("LEFT JOIN video ON playlist_video.vid = video.id").
		Where("playlist_video.playlist_id = ? AND playlist_video.deleted_at IS NULL AND video.deleted_at IS NULL", playlistID).
		Order("playlist_video.sort ASC").
		Limit(pageSize).Offset((page - 1) * pageSize).
		Scan(&list)

	return
}

// GetVideoPlaylists 获取视频所属的公开合集列表
func GetVideoPlaylists(videoId uint) (total int64, list []vo.VideoPlaylistResp) {
	query := global.Mysql.Table("playlist p").
		Joins("INNER JOIN playlist_video pv ON p.id = pv.playlist_id AND pv.deleted_at IS NULL").
		Where("pv.vid = ? AND p.deleted_at IS NULL AND p.is_open = ? AND p.status = ?", videoId, true, global.AUDIT_APPROVED)

	query.Count(&total)
	query.Select("p.id, p.title, p.desc, p.cover, p.uid, p.created_at, p.is_open").
		Order("pv.sort ASC").
		Scan(&list)

	for i := 0; i < len(list); i++ {
		list[i].Author = GetUserBaseInfo(list[i].Uid)
	}

	return
}

// GetVideoPrimaryPlaylist 获取视频的主要合集
func GetVideoPrimaryPlaylist(videoId uint, userId uint) (*vo.VideoPlaylistResp, error) {
	// 获取所有合集
	_, list := GetVideoPlaylists(videoId)

	if len(list) == 0 {
		return nil, errors.New("视频未加入任何合集")
	}

	// 智能选择主合集
	primary := selectPrimaryPlaylist(list, userId)

	return &primary, nil
}

// selectPrimaryPlaylist 智能选择主合集
func selectPrimaryPlaylist(playlists []vo.VideoPlaylistResp, userId uint) vo.VideoPlaylistResp {
	if len(playlists) == 0 {
		return vo.VideoPlaylistResp{}
	}

	// 1. 优先选择用户自己的合集
	for _, playlist := range playlists {
		if playlist.Uid == userId {
			return playlist
		}
	}

	// 2. 选择第一个合集
	return playlists[0]
}
