package service

import (
	"errors"

	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

func ModifyResourceTitle(ctx *gin.Context, modifyTitleReq dto.ModifyResourceTitleReq) error {
	var resource model.Resource
	userId := ctx.GetUint("userId")
	global.Mysql.Model(&model.Resource{}).Where("id = ? and uid = ?", modifyTitleReq.ID, userId).First(&resource)
	if resource.ID == 0 {
		return errors.New("资源不存在")
	}

	if err := global.Mysql.Model(&model.Resource{}).Where("id = ?", modifyTitleReq.ID).Updates(
		map[string]interface{}{
			"title": modifyTitleReq.Title,
		},
	).Error; err != nil {
		utils.ErrorLog("更新资源标题失败", "resource", err.Error())
		return errors.New("更新资源标题失败")
	}

	// 清除视频信息缓存（因为视频信息中包含资源列表）
	cache.DelVideoInfo(resource.Vid)

	return nil
}

// 删除资源
func DeleteResource(ctx *gin.Context, id uint) error {
	var resource model.Resource
	userId := ctx.GetUint("userId")
	global.Mysql.Model(&model.Resource{}).Where("id = ? and uid = ?", id, userId).First(&resource)
	if resource.ID == 0 {
		return errors.New("资源不存在")
	}

	var count int64
	global.Mysql.Model(&model.Resource{}).Where("vid = ?", resource.Vid).Count(&count)
	if count <= 1 {
		return errors.New("至少需要一个视频")
	}

	// 先获取video_index_file记录以获取dir_name，用于删除video_file
	var indexFile model.VideoIndexFile
	global.Mysql.Where("resource_id = ?", id).First(&indexFile)

	if err := global.Mysql.Where("id = ?", id).Delete(&model.Resource{}).Error; err != nil {
		utils.ErrorLog("删除资源失败", "resource", err.Error())
		return errors.New("删除资源失败")
	}

	// 删除关联的m3u8索引文件记录
	if err := global.Mysql.Where("resource_id = ?", id).Delete(&model.VideoIndexFile{}).Error; err != nil {
		utils.ErrorLog("删除m3u8索引文件失败", "resource", err.Error())
	}

	// 删除关联的视频文件记录
	if indexFile.DirName != "" {
		if err := global.Mysql.Where("dir_name = ?", indexFile.DirName).Delete(&model.VideoFile{}).Error; err != nil {
			utils.ErrorLog("删除视频文件记录失败", "resource", err.Error())
		}
	}

	// 删除视频信息缓存（删除后让下次查询时重新从数据库加载）
	cache.DelVideoInfo(resource.Vid)

	return nil
}

// 获取视频资源
func GetVideoResourceByStatus(videoId uint, status int) (resources []vo.ResourceResp) {
	global.Mysql.Model(&model.Resource{}).Where("vid = ? and status = ?", videoId, status).Scan(&resources)

	return
}

// 获取视频资源（含fileId和uid用于全局去重）
func GetReviewResourceList(videoId uint) (resources []vo.ResourceResp) {
	global.Mysql.Model(&model.Resource{}).
		Select("id, created_at, vid, title, duration, status, file_id, uid").
		Where("vid = ?", videoId).Scan(&resources)

	return
}

// GetRelatedResources 获取相同文件的关联稿件（用于审核时查看同文件其他稿件）
func GetRelatedResources(fileID uint, excludeVid uint) (resources []vo.RelatedResourceResp) {
	if fileID == 0 {
		return
	}

	// 查询相同 file_id 的其他资源（排除当前视频）
	global.Mysql.Table("resource").
		Select("resource.id as resource_id, resource.vid, resource.uid, resource.title, resource.status, resource.created_at, users.name as author_name").
		Joins("LEFT JOIN users ON resource.uid = users.id").
		Where("resource.file_id = ? AND resource.vid != ? AND resource.deleted_at IS NULL", fileID, excludeVid).
		Order("resource.created_at ASC").
		Scan(&resources)

	return
}

// 获取视频资源支持的分辨率信息
func GetResourceQuality(ctx *gin.Context, id uint) ([]string, error) {
	if !IsResourceExist(id) {
		return nil, errors.New("资源不存在")
	}

	var quality []string
	if err := global.Mysql.Model(&model.VideoIndexFile{}).Where("resource_id = ?", id).
		Pluck("quality", &quality).Error; err != nil {
		utils.ErrorLog("分辨率信息获取失败", "resource", err.Error())
		return nil, errors.New("获取失败")
	}

	return quality, nil
}

// 获取视频资源支持的分辨率信息(后台管理)
func GetResourceQualityManage(ctx *gin.Context, id uint) ([]string, error) {
	var quality []string
	if err := global.Mysql.Model(&model.VideoIndexFile{}).Where("resource_id = ?", id).
		Pluck("quality", &quality).Error; err != nil {
		utils.ErrorLog("分辨率信息获取失败", "resource", err.Error())
		return nil, errors.New("获取失败")
	}

	return quality, nil
}

func IsResourceExist(resourceId uint) bool {
	var resource model.Resource
	global.Mysql.Model(&model.Resource{}).Select("id").
		Where("id = ? and `status` = ?", resourceId, global.AUDIT_APPROVED).First(&resource)

	return resource.ID != 0
}

// 检查资源是否需要替换（比较hash）
func CheckReplaceResource(ctx *gin.Context, replaceReq dto.ReplaceResourceReq) error {
	userId := ctx.GetUint("userId")

	// 验证资源是否属于当前用户
	var oldResource model.Resource
	global.Mysql.Model(&model.Resource{}).Where("id = ? and uid = ?", replaceReq.ResourceID, userId).First(&oldResource)
	if oldResource.ID == 0 {
		return errors.New("资源不存在")
	}

	// 获取旧资源的索引文件信息
	var oldIndexFile model.VideoIndexFile
	global.Mysql.Where("resource_id = ?", replaceReq.ResourceID).First(&oldIndexFile)

	// 检查哈希值是否相同
	if oldIndexFile.DirName != "" {
		var oldFileInfo model.VideoFile
		global.Mysql.Where("dir_name = ?", oldIndexFile.DirName).First(&oldFileInfo)
		if oldFileInfo.Hash == replaceReq.Hash {
			// 哈希值相同，无需替换
			return errors.New("视频文件相同，无需替换")
		}
	}

	// hash不同，可以替换
	return nil
}

// 替换资源
func ReplaceResource(ctx *gin.Context, replaceReq dto.ReplaceResourceReq) (vo.ResourceResp, error) {
	userId := ctx.GetUint("userId")

	// 验证资源是否属于当前用户
	var oldResource model.Resource
	global.Mysql.Model(&model.Resource{}).Where("id = ? and uid = ?", replaceReq.ResourceID, userId).First(&oldResource)
	if oldResource.ID == 0 {
		return vo.ResourceResp{}, errors.New("资源不存在")
	}

	// 获取旧资源的索引文件信息
	var oldIndexFile model.VideoIndexFile
	global.Mysql.Where("resource_id = ?", replaceReq.ResourceID).First(&oldIndexFile)

	// 获取新视频文件信息
	var newFileInfo model.VideoFile
	if err := global.Mysql.Where("hash = ? and uid = ?", replaceReq.Hash, userId).First(&newFileInfo).Error; err != nil || newFileInfo.ID == 0 {
		utils.ErrorLog("新视频文件信息不存在", "resource", replaceReq.Hash)
		return vo.ResourceResp{}, errors.New("视频文件不存在")
	}

	// 检查哈希值是否相同
	if oldIndexFile.DirName != "" {
		var oldFileInfo model.VideoFile
		global.Mysql.Where("dir_name = ?", oldIndexFile.DirName).First(&oldFileInfo)
		if oldFileInfo.Hash == replaceReq.Hash {
			// 哈希值相同，无需替换
			return vo.ResourceResp{}, errors.New("视频文件相同，无需替换")
		}

		// 检查旧视频文件是否被其他资源使用
		var usageCount int64
		global.Mysql.Model(&model.VideoIndexFile{}).Where("dir_name = ? and resource_id != ?", oldIndexFile.DirName, replaceReq.ResourceID).Count(&usageCount)
		if usageCount == 0 {
			// 没有其他资源使用这个视频文件，可以安全删除
			// 标记旧视频文件记录为删除
			if err := global.Mysql.Where("dir_name = ?", oldIndexFile.DirName).Delete(&model.VideoFile{}).Error; err != nil {
				utils.ErrorLog("删除旧视频文件记录失败", "resource", err.Error())
			}
		}
	}

	// 删除旧的索引文件记录
	if err := global.Mysql.Where("resource_id = ?", replaceReq.ResourceID).Delete(&model.VideoIndexFile{}).Error; err != nil {
		utils.ErrorLog("删除旧m3u8索引文件失败", "resource", err.Error())
	}

	// 读取新视频信息
	uploadVideoPath := "./upload/video/" + newFileInfo.DirName + "/upload.mp4"
	transcodingInfo, err := ProcessVideoInfo(uploadVideoPath)
	if err != nil {
		return vo.ResourceResp{}, errors.New("读取视频信息失败")
	}

	// 更新资源记录
	if err := global.Mysql.Model(&model.Resource{}).Where("id = ?", replaceReq.ResourceID).Updates(
		map[string]interface{}{
			"codec_name": transcodingInfo.CodecName,
			"status":     global.VIDEO_PROCESSING,
			"duration":   transcodingInfo.Duration,
		},
	).Error; err != nil {
		utils.ErrorLog("更新资源记录失败", "resource", err.Error())
		return vo.ResourceResp{}, errors.New("更新资源失败")
	}

	// 启动转码服务
	transcodingInfo.VideoID = oldResource.Vid
	transcodingInfo.DirName = newFileInfo.DirName
	transcodingInfo.ResourceID = replaceReq.ResourceID
	transcodingInfo.OutputDir = "./upload/video/" + newFileInfo.DirName + "/"
	transcodingInfo.InputFile = transcodingInfo.OutputDir + "upload.mp4"
	go VideoTransCoding(transcodingInfo)

	// 清除视频信息缓存
	cache.DelVideoInfo(oldResource.Vid)

	// 返回更新后的资源信息
	var updatedResource model.Resource
	global.Mysql.First(&updatedResource, replaceReq.ResourceID)
	return vo.ResourceToResourceResp(updatedResource), nil
}
