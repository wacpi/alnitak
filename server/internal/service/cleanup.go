package service

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

// CleanupItem 待清理项目
type CleanupItem struct {
	Type   string `json:"type"`   // video, image
	Path   string `json:"path"`   // 本地路径或文件名
	Reason string `json:"reason"` // 清理原因
}

// CleanupResult 清理结果
type CleanupResult struct {
	CleanedVideoDirs  int           `json:"cleanedVideoDirs"`
	CleanedImages     int           `json:"cleanedImages"`
	CleanedVideoFiles int           `json:"cleanedVideoFiles"`
	CleanedIndexFiles int           `json:"cleanedIndexFiles"`
	CleanedImageFiles int           `json:"cleanedImageFiles"`
	CleanedResources  int           `json:"cleanedResources"`
	Errors            []string      `json:"errors"`
	DryRun            bool          `json:"dryRun"`
	Items             []CleanupItem `json:"items"` // 待清理/已清理的文件列表
}

// CleanupOrphanedResources 清理孤立资源
// dryRun: 如果为true，只返回将要清理的内容，不实际执行删除
func CleanupOrphanedResources(dryRun bool) CleanupResult {
	result := CleanupResult{
		Errors: make([]string, 0),
		Items:  make([]CleanupItem, 0),
		DryRun: dryRun,
	}

	// 1. 清理孤立的视频目录（基于完整关联校验）
	result.cleanOrphanedVideoDirs(dryRun)

	// 2. 清理孤立的图片文件
	result.cleanOrphanedImages(dryRun)

	return result
}

// cleanOrphanedVideoDirs 清理孤立的视频目录
// 清理条件：本地视频目录对应的资源在数据库中不存在有效引用
// 校验链路：Video -> Resource -> VideoIndexFile(dirName) / VideoFile(dirName)
func (r *CleanupResult) cleanOrphanedVideoDirs(dryRun bool) {
	videoDir := "./upload/video"
	if !utils.IsFileExists(videoDir) {
		return
	}

	entries, err := os.ReadDir(videoDir)
	if err != nil {
		r.Errors = append(r.Errors, "读取视频目录失败: "+err.Error())
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()

		// 跳过 .gitkeep 等隐藏文件/目录
		if strings.HasPrefix(dirName, ".") {
			continue
		}

		// 检查这个目录是否有有效的关联
		reason := checkVideoDirValidity(dirName)
		if reason == "" {
			continue // 有效，保留
		}

		// 需要清理
		localDir := filepath.Join(videoDir, dirName)
		r.Items = append(r.Items, CleanupItem{
			Type:   "video",
			Path:   dirName,
			Reason: reason,
		})

		if !dryRun {
			// 删除OSS上的文件
			deleteVideoFromOSS(localDir, &r.Errors)
			// 删除本地目录
			if err := os.RemoveAll(localDir); err != nil {
				r.Errors = append(r.Errors, "删除视频目录失败: "+localDir+" - "+err.Error())
			} else {
				r.CleanedVideoDirs++
				utils.InfoLog("已删除视频目录: "+localDir+" 原因: "+reason, "cleanup")
			}

			// 清理相关的数据库记录
			cleanVideoDirDbRecords(dirName, r)
		} else {
			r.CleanedVideoDirs++
		}
	}
}

// checkVideoDirValidity 检查视频目录是否有效
// 返回空字符串表示有效，返回原因字符串表示需要清理
// 注意：同一个dirName可能有多条记录（不同画质），只要有一条是有效的，整个目录就应该保留
func checkVideoDirValidity(dirName string) string {
	// 1. 检查 VideoIndexFile 表（获取所有记录）
	var indexFiles []model.VideoIndexFile
	global.Mysql.Unscoped().Where("dir_name = ?", dirName).Find(&indexFiles)

	if len(indexFiles) == 0 {
		// VideoIndexFile 不存在，检查 VideoFile
		var videoFile model.VideoFile
		global.Mysql.Unscoped().Where("dir_name = ?", dirName).First(&videoFile)

		if videoFile.ID == 0 {
			return "数据库无记录"
		}
		// VideoFile 存在但 VideoIndexFile 不存在，可能是上传中断，保留
		return ""
	}

	// 2. 遍历所有 VideoIndexFile 记录，只要有一条完整有效的链路，就保留目录
	var lastReason string
	for _, indexFile := range indexFiles {
		// 检查这条记录是否有效
		reason := checkSingleIndexFileValidity(indexFile)
		if reason == "" {
			// 找到一条有效记录，目录有效，保留
			return ""
		}
		lastReason = reason
	}

	// 所有记录都无效，返回最后一个无效原因
	return lastReason
}

// checkSingleIndexFileValidity 检查单条 VideoIndexFile 记录是否有效
func checkSingleIndexFileValidity(indexFile model.VideoIndexFile) string {
	// 检查 VideoIndexFile 是否被软删除
	if indexFile.DeletedAt.Valid {
		return "VideoIndexFile已删除"
	}

	// 检查对应的 Resource
	var resource model.Resource
	global.Mysql.Unscoped().Where("id = ?", indexFile.ResourceID).First(&resource)

	if resource.ID == 0 {
		return "Resource记录不存在"
	}

	if resource.DeletedAt.Valid {
		return "Resource已删除"
	}

	// 检查对应的 Video
	var video model.Video
	global.Mysql.Unscoped().Where("id = ?", resource.Vid).First(&video)

	if video.ID == 0 {
		return "Video记录不存在"
	}

	if video.DeletedAt.Valid {
		return "Video已删除"
	}

	// 这条记录完整有效
	return ""
}

// cleanVideoDirDbRecords 清理视频目录相关的数据库记录
func cleanVideoDirDbRecords(dirName string, r *CleanupResult) {
	// 删除 VideoIndexFile 记录
	result := global.Mysql.Unscoped().Where("dir_name = ?", dirName).Delete(&model.VideoIndexFile{})
	r.CleanedIndexFiles += int(result.RowsAffected)

	// 删除 VideoFile 记录
	result = global.Mysql.Unscoped().Where("dir_name = ?", dirName).Delete(&model.VideoFile{})
	r.CleanedVideoFiles += int(result.RowsAffected)

	// 查找并删除相关的 Resource 记录
	var indexFiles []model.VideoIndexFile
	global.Mysql.Unscoped().Where("dir_name = ?", dirName).Find(&indexFiles)
	for _, indexFile := range indexFiles {
		if indexFile.ResourceID > 0 {
			result = global.Mysql.Unscoped().Where("id = ?", indexFile.ResourceID).Delete(&model.Resource{})
			r.CleanedResources += int(result.RowsAffected)
		}
	}
}

// cleanOrphanedImages 清理孤立的图片文件
// 清理条件（保守策略）：
// 1. 本地文件存在但数据库ImageFile表中无记录（真正的孤立文件）
// 2. 且该图片没有被任何内容引用
// 注意：如果ImageFile表中有记录，说明是正常上传的图片，即使暂时没被引用也保留
func (r *CleanupResult) cleanOrphanedImages(dryRun bool) {
	imageDir := "./upload/image"
	if !utils.IsFileExists(imageDir) {
		return
	}

	entries, err := os.ReadDir(imageDir)
	if err != nil {
		r.Errors = append(r.Errors, "读取图片目录失败: "+err.Error())
		return
	}

	// 支持的图片扩展名
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".bmp": true, ".ico": true, ".svg": true,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()

		// 只处理图片文件，跳过 .gitkeep 等非图片文件
		ext := strings.ToLower(filepath.Ext(fileName))
		if !imageExts[ext] {
			continue
		}

		// 检查数据库ImageFile表中是否有记录
		var imageFileCount int64
		global.Mysql.Model(&model.ImageFile{}).Where("file_name = ?", fileName).Count(&imageFileCount)

		// 如果数据库有记录，说明是正常上传的图片，保留不删除
		if imageFileCount > 0 {
			continue
		}

		// 数据库无记录，检查是否被任何内容引用
		imageUrl := "/api/image/" + fileName
		if isImageReferenced(imageUrl) {
			continue
		}

		// 数据库无记录且无引用，可以安全删除
		localPath := filepath.Join(imageDir, fileName)
		r.Items = append(r.Items, CleanupItem{
			Type:   "image",
			Path:   fileName,
			Reason: "无数据库记录且无引用",
		})

		if !dryRun {
			// 删除OSS上的图片
			if global.Config.Storage.OssType != "local" {
				objectKey := "image/" + fileName
				utils.InfoLog("删除OSS图片: "+objectKey, "cleanup")
				if err := global.Storage.DeleteObject(objectKey); err != nil {
					r.Errors = append(r.Errors, "删除OSS图片失败: "+objectKey+" - "+err.Error())
				}
			}
			// 删除本地文件
			if err := os.Remove(localPath); err != nil {
				r.Errors = append(r.Errors, "删除孤立图片失败: "+localPath+" - "+err.Error())
			} else {
				r.CleanedImages++
				utils.InfoLog("已删除孤立图片(无数据库记录且无引用): "+localPath, "cleanup")
			}
		} else {
			r.CleanedImages++
		}
	}
}

// isImageReferenced 检查图片是否被引用
func isImageReferenced(imageUrl string) bool {
	// 30天前的时间（软删除30天内的数据也要保留图片）
	expireTime := time.Now().AddDate(0, 0, -30)

	// 检查Video.cover（包括30天内软删除的）
	var videoCount int64
	global.Mysql.Unscoped().Model(&model.Video{}).
		Where("cover = ? AND (deleted_at IS NULL OR deleted_at > ?)", imageUrl, expireTime).
		Count(&videoCount)
	if videoCount > 0 {
		return true
	}

	// 检查Article.cover
	var articleCount int64
	global.Mysql.Unscoped().Model(&model.Article{}).
		Where("cover = ? AND (deleted_at IS NULL OR deleted_at > ?)", imageUrl, expireTime).
		Count(&articleCount)
	if articleCount > 0 {
		return true
	}

	// 检查User.avatar和space_cover（用户不会软删除）
	var userCount int64
	global.Mysql.Model(&model.User{}).
		Where("avatar = ? OR space_cover = ?", imageUrl, imageUrl).
		Count(&userCount)
	if userCount > 0 {
		return true
	}

	// 检查Carousel.img（轮播图不会软删除）
	var carouselCount int64
	global.Mysql.Model(&model.Carousel{}).
		Where("img = ?", imageUrl).
		Count(&carouselCount)
	if carouselCount > 0 {
		return true
	}

	return false
}

// deleteVideoFromOSS 删除OSS上的视频文件
func deleteVideoFromOSS(localDir string, errors *[]string) {
	if global.Config.Storage.OssType == "local" {
		return
	}

	// 从localDir中提取目录名 (例如: "./upload/video/abc123" -> "abc123")
	dirName := filepath.Base(localDir)
	if dirName == "" || dirName == "." {
		return
	}

	// 遍历本地目录，删除OSS上对应的文件
	filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// 直接使用 video/dirName/fileName 格式构建objectKey
		fileName := filepath.Base(path)
		objectKey := "video/" + dirName + "/" + fileName

		utils.InfoLog("删除OSS文件: "+objectKey, "cleanup")
		if err := global.Storage.DeleteObject(objectKey); err != nil {
			errMsg := "删除OSS文件失败: " + objectKey + " - " + err.Error()
			utils.ErrorLog(errMsg, "cleanup", err.Error())
			if errors != nil {
				*errors = append(*errors, errMsg)
			}
		}
		return nil
	})
}

// GetCleanupPreview 获取清理预览（不执行实际删除）
func GetCleanupPreview() CleanupResult {
	return CleanupOrphanedResources(true)
}

// ExecuteCleanup 执行清理
func ExecuteCleanup() CleanupResult {
	return CleanupOrphanedResources(false)
}
