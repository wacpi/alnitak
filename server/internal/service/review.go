package service

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

// 审核通过(视频)
func ReviewVideoApproved(ctx *gin.Context, reviewVideoReq dto.ReviewVideoReq) error {
	tx := global.Mysql.Begin()
	// 只把待审核的资源改为审核通过（不影响转码中、已通过等其他状态的资源）
	if err := tx.Model(&model.Resource{}).
		Where("vid = ? AND status = ?", reviewVideoReq.Vid, global.WAITING_REVIEW).
		Updates(map[string]interface{}{"status": global.AUDIT_APPROVED}).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新资源状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	// 统计已审核通过的资源时长（不含转码中/失败的）
	var duration float64
	tx.Model(&model.Resource{}).Where("vid = ? AND status = ?", reviewVideoReq.Vid, global.AUDIT_APPROVED).
		Pluck("SUM(duration) as duration", &duration)

	// 更新视频状态为审核通过
	if err := tx.Model(&model.Video{}).Where("id = ?", reviewVideoReq.Vid).Updates(map[string]interface{}{
		"status":   global.AUDIT_APPROVED,
		"duration": duration,
	}).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新视频状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	// 添加审核记录
	if err := tx.Create(&model.Review{
		Cid:    reviewVideoReq.Vid,
		Status: global.AUDIT_APPROVED,
		Remark: "",
		Uid:    ctx.GetUint("userId"),
		Type:   global.CONTENT_TYPE_VIDEO,
	}).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新视频状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	// 先提交事务
	if err := tx.Commit().Error; err != nil {
		utils.ErrorLog("提交事务失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	// 事务成功后再操作缓存
	video, _ := FindVideoById(reviewVideoReq.Vid)
	// 先清除视频信息缓存（让下次查询时重新从数据库加载最新数据）
	cache.DelVideoInfo(reviewVideoReq.Vid)
	// 再添加视频ID到redis分区列表
	cache.SetVideoId(global.VideoPartitionMap[video.PartitionId], video.ID)

	return nil
}

// 审核不通过(视频)
func ReviewVideoFailed(ctx *gin.Context, reviewVideoReq dto.ReviewVideoReq) error {
	tx := global.Mysql.Begin()
	// 更新视频状态为审核不通过
	if err := tx.Model(&model.Video{}).Where("id = ?", reviewVideoReq.Vid).Updates(
		map[string]interface{}{"status": global.REVIEW_FAILED},
	).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新视频状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	// 如果指定了冲突稿件，更新资源表的冲突关联字段
	if reviewVideoReq.ConflictResourceID != 0 {
		// 更新该视频下所有资源的冲突关联
		if err := tx.Model(&model.Resource{}).Where("vid = ?", reviewVideoReq.Vid).Updates(
			map[string]interface{}{
				"conflict_resource_id": reviewVideoReq.ConflictResourceID,
				"conflict_reason":      reviewVideoReq.ConflictReason,
			},
		).Error; err != nil {
			tx.Rollback()
			utils.ErrorLog("更新冲突关联失败", "review", err.Error())
			return errors.New("更新冲突关联失败")
		}
		utils.InfoLog(fmt.Sprintf("【审核驳回】设置冲突关联 vid=%d -> conflictResourceId=%d", reviewVideoReq.Vid, reviewVideoReq.ConflictResourceID), "review")
	}

	// 添加审核记录
	if err := tx.Create(&model.Review{
		Cid:    reviewVideoReq.Vid,
		Status: reviewVideoReq.Status,
		Remark: reviewVideoReq.Remark,
		Uid:    ctx.GetUint("userId"),
		Type:   global.CONTENT_TYPE_VIDEO,
	}).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新视频状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	tx.Commit()

	// 清除视频信息缓存和视频ID缓存
	video, _ := FindVideoById(reviewVideoReq.Vid)
	if video.ID != 0 {
		cache.DelVideoInfo(reviewVideoReq.Vid)
		cache.DelVideoId(video.PartitionId, reviewVideoReq.Vid)
	}

	return nil
}

// 获取审核记录(视频)
func GetVideoReviewRecord(ctx *gin.Context, videoId uint) (vo.ReviewResp, error) {
	userId := ctx.GetUint("userId")
	video, _ := FindVideoById(videoId)
	if video.Uid != userId {
		return vo.ReviewResp{}, errors.New("视频不存在")
	}

	var review vo.ReviewResp
	global.Mysql.Model(&model.Review{}).Select(vo.REVIEW_FIELD).
		Where("cid = ? and `type` = ?", videoId, global.CONTENT_TYPE_VIDEO).Last(&review)

	return review, nil
}

// 审核通过(文章)
func ReviewArticleApproved(ctx *gin.Context, reviewArticleReq dto.ReviewArticleReq) error {
	tx := global.Mysql.Begin()

	// 更新文章状态为审核通过
	if err := tx.Model(&model.Article{}).Where("id = ?", reviewArticleReq.Aid).Updates(
		map[string]interface{}{"status": global.AUDIT_APPROVED},
	).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新文章状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	// 添加审核记录
	if err := tx.Create(&model.Review{
		Cid:    reviewArticleReq.Aid,
		Status: global.AUDIT_APPROVED,
		Remark: "",
		Uid:    ctx.GetUint("userId"),
		Type:   global.CONTENT_TYPE_ARTICLE,
	}).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新视频状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	// 文章ID添加到redis中
	cache.SetArticleId(reviewArticleReq.Aid)

	tx.Commit()
	return nil
}

// 审核不通过(文章)
func ReviewArticleFailed(ctx *gin.Context, reviewArticleReq dto.ReviewArticleReq) error {
	tx := global.Mysql.Begin()
	// 更新文章状态为审核不通过
	if err := tx.Model(&model.Article{}).Where("id = ?", reviewArticleReq.Aid).Updates(
		map[string]interface{}{"status": global.REVIEW_FAILED},
	).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新文章状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	// 添加审核记录
	if err := tx.Create(&model.Review{
		Cid:    reviewArticleReq.Aid,
		Status: reviewArticleReq.Status,
		Remark: reviewArticleReq.Remark,
		Uid:    ctx.GetUint("userId"),
		Type:   global.CONTENT_TYPE_ARTICLE,
	}).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新文章状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	tx.Commit()
	return nil
}

// 获取审核记录(文章)
func GetArticleReviewRecord(ctx *gin.Context, articleId uint) (vo.ReviewResp, error) {
	userId := ctx.GetUint("userId")
	article, _ := FindArticleById(articleId)
	if article.Uid != userId {
		return vo.ReviewResp{}, errors.New("内容不存在")
	}

	var review vo.ReviewResp
	global.Mysql.Model(&model.Review{}).Select(vo.REVIEW_FIELD).
		Where("cid = ? and `type` = ?", articleId, global.CONTENT_TYPE_ARTICLE).Last(&review)

	return review, nil
}

// 获取待审核合集列表
func GetReviewPlaylistList(page, pageSize int) (total int64, list []vo.PlaylistResp) {
	global.Mysql.Model(&model.Playlist{}).Where("status = ?", global.WAITING_REVIEW).Count(&total)
	global.Mysql.Model(&model.Playlist{}).Select(vo.PLAYLIST_FIELD).
		Where("status = ?", global.WAITING_REVIEW).
		Order("created_at asc").
		Limit(pageSize).Offset((page - 1) * pageSize).Scan(&list)

	// 填充作者信息
	for i := range list {
		list[i].Author = GetUserBaseInfo(list[i].Uid)
	}
	return
}

// 审核通过(合集)
func ReviewPlaylistApproved(ctx *gin.Context, req dto.ReviewPlaylistReq) error {
	tx := global.Mysql.Begin()
	if err := tx.Model(&model.Playlist{}).Where("id = ?", req.ID).Updates(
		map[string]interface{}{"status": global.AUDIT_APPROVED},
	).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新合集状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	if err := tx.Create(&model.Review{
		Cid:    req.ID,
		Status: global.AUDIT_APPROVED,
		Remark: "",
		Uid:    ctx.GetUint("userId"),
		Type:   global.CONTENT_TYPE_PLAYLIST,
	}).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("创建审核记录失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	tx.Commit()
	return nil
}

// 审核不通过(合集)
func ReviewPlaylistFailed(ctx *gin.Context, req dto.ReviewPlaylistReq) error {
	tx := global.Mysql.Begin()
	if err := tx.Model(&model.Playlist{}).Where("id = ?", req.ID).Updates(
		map[string]interface{}{"status": global.REVIEW_FAILED},
	).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("更新合集状态失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	if err := tx.Create(&model.Review{
		Cid:    req.ID,
		Status: req.Status,
		Remark: req.Remark,
		Uid:    ctx.GetUint("userId"),
		Type:   global.CONTENT_TYPE_PLAYLIST,
	}).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("创建审核记录失败", "review", err.Error())
		return errors.New("更新状态失败")
	}

	tx.Commit()
	return nil
}

// 获取审核记录(合集)
func GetPlaylistReviewRecord(ctx *gin.Context, playlistId uint) (vo.ReviewResp, error) {
	userId := ctx.GetUint("userId")
	var playlist model.Playlist
	if err := global.Mysql.First(&playlist, playlistId).Error; err != nil {
		return vo.ReviewResp{}, errors.New("合集不存在")
	}
	if playlist.Uid != userId {
		return vo.ReviewResp{}, errors.New("合集不存在")
	}

	var review vo.ReviewResp
	global.Mysql.Model(&model.Review{}).Select(vo.REVIEW_FIELD).
		Where("cid = ? and `type` = ?", playlistId, global.CONTENT_TYPE_PLAYLIST).Last(&review)

	return review, nil
}
