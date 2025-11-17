package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

func UploadVideoInfo(ctx *gin.Context, uploadVideoReq dto.UploadVideoReq) error {
	userId := ctx.GetUint("userId")
	v, _ := FindVideoById(uploadVideoReq.Vid)
	if cache.GetUploadImage(uploadVideoReq.Cover) != userId {
		// 查询是否与旧封面图一致
		if v.Cover != uploadVideoReq.Cover {
			return errors.New("文件链接无效")
		}
	}

	if v.PartitionId != 0 {
		return errors.New("视频信息已存在")
	}

	if !IsSubpartition(uploadVideoReq.PartitionId, global.CONTENT_TYPE_VIDEO) {
		return errors.New("分区不存在")
	}

	if err := global.Mysql.Model(&model.Video{}).Where("id = ?", uploadVideoReq.Vid).Updates(
		map[string]interface{}{
			"title":        uploadVideoReq.Title,
			"cover":        uploadVideoReq.Cover,
			"desc":         uploadVideoReq.Desc,
			"tags":         uploadVideoReq.Tags,
			"copyright":    uploadVideoReq.Copyright,
			"partition_id": uploadVideoReq.PartitionId,
			"status":       getVideoStatus(uploadVideoReq.Vid),
		},
	).Error; err != nil {
		utils.ErrorLog("修改视频失败", "video", err.Error())
		return errors.New("修改失败")
	}

	// 上传视频信息后删除缓存，让下次查询时重新加载最新数据
	cache.DelVideoInfo(uploadVideoReq.Vid)

	return nil
}

func GetVideoStatus(ctx *gin.Context, vid uint) (video vo.VideoStatusResp, err error) {
	userId := ctx.GetUint("userId")
	global.Mysql.Model(&model.Video{}).Select(vo.VIDEO_STATUS_FIELD).Where("id = ? and uid = ?", vid, userId).Scan(&video)
	if video.ID == 0 {
		return video, errors.New("视频不存在")
	}

	//查询分区下的视频资源
	video.Resources = GetReviewResourceList(vid)

	return video, nil
}

// 获取视频文件
func GetVideoFile(ctx *gin.Context, resourceId uint, quality string) (string, error) {
	if !IsResourceExist(resourceId) {
		return "", errors.New("资源不存在")
	}

	var file model.VideoIndexFile
	global.Mysql.Where("resource_id = ? and quality = ?", resourceId, quality).First(&file)

	res := ""
	key := uuid.New().String()
	cache.SetVideoSlice(key, file.DirName)
	for _, line := range strings.Split(file.Content, "\n") {
		//根据关键词覆盖当前行
		if strings.Contains(line, ".ts") {
			res += "/api/v1/video/slice/" + line + "?key=" + key + "\n"
		} else {
			res += line + "\n"
		}
	}

	return res, nil
}

// 获取视频文件（后台管理）
func GetVideoFileManage(ctx *gin.Context, resourceId uint, quality string) (string, error) {
	var file model.VideoIndexFile
	global.Mysql.Where("resource_id = ? and quality = ?", resourceId, quality).First(&file)

	res := ""
	key := uuid.New().String()
	cache.SetVideoSlice(key, file.DirName)
	for _, line := range strings.Split(file.Content, "\n") {
		//根据关键词覆盖当前行
		if strings.Contains(line, ".ts") {
			res += "/api/v1/video/slice/" + line + "?key=" + key + "\n"
		} else {
			res += line + "\n"
		}
	}

	return res, nil
}

// 获取视频切所在文件目录
func GetVideoSliceDir(key string) string {
	return cache.GetVideoSlice(key)
}

// 获取自己上传的视频
func GetUploadVideoList(ctx *gin.Context, page, pageSize int) (total int64, videos []vo.UploadVideoResp) {
	userId := ctx.GetUint("userId")

	global.Mysql.Model(&model.Video{}).Where("uid = ?", userId).Count(&total)
	global.Mysql.Model(&model.Video{}).Select(vo.UPLOAD_VIDEO_FIELD).
		Where("uid = ?", userId).Limit(pageSize).Offset((page - 1) * pageSize).Scan(&videos)

	// 更新播放量数据
	for i := 0; i < len(videos); i++ {
		videos[i].Clicks += GetVideoClicks(videos[i].ID)
	}

	return
}

func EditVideoInfo(ctx *gin.Context, editVideoReq dto.EditVideoReq) error {
	userId := ctx.GetUint("userId")
	oldVideo, _ := FindVideoById(editVideoReq.Vid)
	if cache.GetUploadImage(editVideoReq.Cover) != userId {
		// 查询是否与旧封面图一致
		if oldVideo.Cover != editVideoReq.Cover {
			return errors.New("文件链接无效")
		}
	}

	// 准备更新的字段
	updateData := map[string]any{
		"title": editVideoReq.Title,
		"cover": editVideoReq.Cover,
		"desc":  editVideoReq.Desc,
		"tags":  editVideoReq.Tags,
	}

	// 重新计算视频状态（所有编辑都需要重新审核，防止替换违规内容）
	newStatus := getVideoStatus(editVideoReq.Vid)

	// 特殊处理：如果视频原本已审核通过，编辑后需要重新审核
	// 设置为WAITING_REVIEW(500)而不是根据资源状态计算，避免因为有转码中资源而变成SUBMIT_REVIEW(300)
	if oldVideo.Status == global.AUDIT_APPROVED {
		newStatus = global.WAITING_REVIEW
		utils.InfoLog(fmt.Sprintf("已发布视频被编辑，VideoID=%d，状态从AUDIT_APPROVED(0)改为WAITING_REVIEW(500)，需要重新审核", editVideoReq.Vid), "video")
	}

	updateData["status"] = newStatus

	if err := global.Mysql.Model(&model.Video{}).Where("id = ?", editVideoReq.Vid).Updates(updateData).Error; err != nil {
		utils.ErrorLog("修改视频失败", "video", err.Error())
		return errors.New("修改失败")
	}

	// 如果是已发布视频被编辑，需要从分区列表中移除（因为状态变为待审核）
	if oldVideo.Status == global.AUDIT_APPROVED {
		cache.DelVideoId(oldVideo.PartitionId, oldVideo.ID)
	}

	// 删除视频信息缓存
	cache.DelVideoInfo(editVideoReq.Vid)

	return nil
}

func DeleteVideo(ctx *gin.Context, id uint) error {
	var video model.Video
	userId := ctx.GetUint("userId")
	global.Mysql.Model(&model.Video{}).Where("id = ? and uid = ?", id, userId).First(&video)
	if video.ID == 0 {
		return errors.New("视频不存在")
	}

	if err := global.Mysql.Where("id = ?", id).Delete(&model.Video{}).Error; err != nil {
		utils.ErrorLog("删除视频失败", "video", err.Error())
		return errors.New("删除视频失败")
	}

	// 清理所有相关缓存
	// 1. 删除分区视频列表中的视频ID
	cache.DelVideoId(video.PartitionId, video.ID)

	// 2. 删除热门视频列表中的视频ID
	cache.DelSingleHotVideoId(video.ID)

	// 3. 删除视频信息缓存
	cache.DelVideoInfo(id)

	return nil
}

// 获取视频信息
func GetVideoById(ctx *gin.Context, videoId uint) (vo.VideoResp, error) {
	video := GetVideoInfo(videoId)
	if video.ID == 0 {
		return video, errors.New("视频信息不存在")
	}

	// 增加播放量(一个ip在同一个视频下，每30分钟可重新增加1播放量)
	AddVideoClicks(videoId, ctx.ClientIP())
	video.Clicks += GetVideoClicks(video.ID)

	return video, nil
}

// 获取所有的视频列表
func GetAllVideoList(ctx *gin.Context) (videos []vo.AllVideoResp) {
	userId := ctx.GetUint("userId")
	global.Mysql.Model(&model.Video{}).Select("`id`,`title`").Where("uid = ?", userId).Scan(&videos)

	return
}

// 获取用户视频
func GetVideoByUser(ctx *gin.Context, userId uint, page, pageSize int) (total int64, videos []vo.UploadVideoResp) {
	global.Mysql.Model(&model.Video{}).
		Where("uid = ? and status = ?", userId, global.AUDIT_APPROVED).Count(&total)
	global.Mysql.Model(&model.Video{}).Select(vo.UPLOAD_VIDEO_FIELD).
		Where("uid = ? and status = ?", userId, global.AUDIT_APPROVED).
		Limit(pageSize).Offset((page - 1) * pageSize).Scan(&videos)

	// 更新播放量数据
	for i := 0; i < len(videos); i++ {
		videos[i].Clicks += GetVideoClicks(videos[i].ID)
	}

	return
}

// 获取视频列表(后台管理)
func GetVideoListManage(videoListReq dto.VideoListReq) (total int64, videos []vo.VideoInfoManageResp) {
	global.Mysql.Model(&model.Video{}).Where("status = ?", global.AUDIT_APPROVED).Count(&total)
	global.Mysql.Model(&model.Video{}).Where("status = ?", global.AUDIT_APPROVED).
		Limit(videoListReq.PageSize).Offset((videoListReq.Page - 1) * videoListReq.PageSize).Scan(&videos)

	// 更新播放量和作者数据
	for i := 0; i < len(videos); i++ {
		videos[i].Clicks += GetVideoClicks(videos[i].ID)
		videos[i].Author = GetUserBaseInfo(videos[i].Uid)
	}

	return
}

// 删除视频(后台管理)
func DeleteVideoManage(ctx *gin.Context, id uint) error {
	// 先查询视频信息，用于删除缓存
	var video model.Video
	global.Mysql.Model(&model.Video{}).Where("id = ?", id).First(&video)

	if err := global.Mysql.Where("id = ?", id).Delete(&model.Video{}).Error; err != nil {
		utils.ErrorLog("删除视频失败", "video", err.Error())
		return errors.New("删除视频失败")
	}

	// 清理所有相关缓存
	if video.ID != 0 {
		// 1. 删除分区视频列表中的视频ID
		cache.DelVideoId(video.PartitionId, id)

		// 2. 删除热门视频列表中的视频ID
		cache.DelSingleHotVideoId(id)
	}

	// 3. 删除视频信息缓存
	cache.DelVideoInfo(id)

	return nil
}

// 获取待审核视频列表
func GetReviewList(reviewListReq dto.ReviewListReq) (total int64, videos []vo.ReviewListResp) {
	global.Mysql.Model(&model.Video{}).Where("status = ?", global.WAITING_REVIEW).Count(&total)
	global.Mysql.Model(&model.Video{}).Where("status = ?", global.WAITING_REVIEW).
		Limit(reviewListReq.PageSize).Offset((reviewListReq.Page - 1) * reviewListReq.PageSize).Scan(&videos)

	// 更新播放量和作者数据
	for i := 0; i < len(videos); i++ {
		videos[i].Author = GetUserBaseInfo(videos[i].Uid)
	}

	return
}

// 获取热门视频
func GetHotVideo(ctx *gin.Context, page, pageSize int) []vo.VideoResp {
	ids := cache.GetHotVideoId()
	videoIds := utils.SlicePagingStr(ids, page, pageSize)

	len := len(videoIds)
	videos := make([]vo.VideoResp, len)
	for i := 0; i < len; i++ {
		id := utils.StringToUint(videoIds[i])
		if id == 0 {
			continue
		}
		videos[i] = GetVideoInfo(id)
		// 同步播放量
		videos[i].Clicks += GetVideoClicks(id)
	}

	return videos
}

// 获取分区视频
func GetVideoListByPartition(ctx *gin.Context, size int, partitionId uint) []vo.VideoResp {
	videoIds := cache.GetVideoIdByPartition(partitionId, int64(size))

	len := len(videoIds)
	videos := make([]vo.VideoResp, len)
	for i := 0; i < len; i++ {
		id := utils.StringToUint(videoIds[i])
		if id == 0 {
			continue
		}
		videos[i] = GetVideoInfo(id)
		// 同步播放量
		videos[i].Clicks += GetVideoClicks(id)
	}

	return videos
}

// 获取相关推荐视频
func GetRelatedVideoList(ctx *gin.Context, videoId uint) []vo.VideoResp {
	video := GetVideoInfo(videoId)

	var videoIds []uint
	// 查询同作者的2个视频
	var authorVideoIds []uint
	global.Mysql.Model(&model.Video{}).
		Where("uid = ? and id != ? and `status` = ?", video.Uid, videoId, global.AUDIT_APPROVED).
		Limit(2).Pluck("id", &authorVideoIds)
	videoIds = append(videoIds, authorVideoIds...)

	// 查询同分区的7个视频（防止视频与同作者视频相同或者与当前视频相同）
	// 获取主分区ID用于推荐
	partitionId := video.PartitionId
	if parentId, exists := global.VideoPartitionMap[video.PartitionId]; exists && parentId != 0 {
		partitionId = parentId
	}

	for _, v := range cache.GetVideoIdByPartition(partitionId, 7) {
		id := utils.StringToUint(v)
		if id != videoId && !utils.IsUintInSlice(authorVideoIds, id) {
			videoIds = append(videoIds, id)
		}
	}

	len := len(videoIds)
	videos := make([]vo.VideoResp, len)
	for i := 0; i < len; i++ {
		videos[i] = GetVideoInfo(videoIds[i])
		// 同步播放量
		videos[i].Clicks += GetVideoClicks(videoIds[i])
	}

	return videos
}

// 获取相关推荐视频
func SearchVideo(ctx *gin.Context, searchVideoReq dto.SearchVideoReq) []vo.VideoResp {
	var videoIds []uint
	if len(searchVideoReq.KeyWords) == 0 {
		global.Mysql.Model(&model.Video{}).Where("`status` = ?", global.AUDIT_APPROVED).
			Limit(searchVideoReq.PageSize).Offset((searchVideoReq.Page-1)*searchVideoReq.PageSize).Pluck("id", &videoIds)
	} else {
		// 直接用mysql模糊查询，之后可能会更换为es
		keywords := "%" + searchVideoReq.KeyWords + "%"
		global.Mysql.Model(&model.Video{}).Where("`status` = ? and (title like ? or tags like ?)", global.AUDIT_APPROVED, keywords, keywords).
			Limit(searchVideoReq.PageSize).Offset((searchVideoReq.Page-1)*searchVideoReq.PageSize).Pluck("id", &videoIds)
	}

	len := len(videoIds)
	videos := make([]vo.VideoResp, len)
	for i := 0; i < len; i++ {
		videos[i] = GetVideoInfo(videoIds[i])
		// 同步播放量
		videos[i].Clicks += GetVideoClicks(videoIds[i])
	}

	return videos
}

func CreateVideo(video *model.Video) (uint, error) {
	if err := global.Mysql.Create(video).Error; err != nil {
		utils.ErrorLog("创建视频失败", "video", err.Error())
		return 0, errors.New("创建视频失败")
	}

	return video.ID, nil
}

// 通过视频ID查询视频
func FindVideoById(id uint) (video model.Video, err error) {
	err = global.Mysql.Where("`id` = ?", id).First(&video).Error
	return
}

// 获取视频信息
func GetVideoInfo(videoId uint) (video vo.VideoResp) {
	video = cache.GetVideoInfo(videoId)
	if video.ID == 0 {
		// 缓存不存在，从数据库加载并写入缓存
		video = VideoWriteCache(videoId)
	}
	// 直接使用缓存中的完整数据，不再重新查询Resources和Author
	// 如果需要最新数据，应该先调用cache.DelVideoInfo删除缓存，再调用本函数

	return
}

// 视频信息写入缓存
func VideoWriteCache(videoId uint) (video vo.VideoResp) {
	global.Mysql.Model(&model.Video{}).Select(vo.VIDEO_FIELD).
		Where("id = ? and status = ?", videoId, global.AUDIT_APPROVED).Scan(&video)
	if video.ID == 0 {
		utils.ErrorLog("视频信息不存在", "video", "")
		return
	}

	// 获取作者信息
	video.Author = GetUserBaseInfo(video.Uid)
	// 获取视频资源
	video.Resources = GetVideoResourceByStatus(videoId, global.AUDIT_APPROVED)
	// 确保Resources不是nil，如果是nil则初始化为空切片，避免JSON序列化为null
	if video.Resources == nil {
		video.Resources = []vo.ResourceResp{}
	}

	// 存到redis
	cache.SetVideoInfo(video)

	return
}

// 获取视频状态
func getVideoStatus(videoId uint) int {
	var processingCount int64 // 转码中的资源数量
	var failedCount int64      // 转码失败的资源数量
	var totalCount int64       // 总资源数量

	global.Mysql.Model(&model.Resource{}).Where("vid = ?", videoId).Count(&totalCount)
	global.Mysql.Model(&model.Resource{}).Where("vid = ? and status = ?", videoId, global.VIDEO_PROCESSING).Count(&processingCount)
	global.Mysql.Model(&model.Resource{}).Where("vid = ? and status = ?", videoId, global.PROCESSING_FAIL).Count(&failedCount)

	// 如果所有资源都失败了，返回处理失败状态
	if failedCount == totalCount && totalCount > 0 {
		return global.PROCESSING_FAIL
	}

	// 如果还有转码中的资源，返回提交审核状态
	if processingCount > 0 {
		return global.SUBMIT_REVIEW
	}

	// 所有资源都完成了（至少有一个成功），返回待审核状态
	return global.WAITING_REVIEW
}
