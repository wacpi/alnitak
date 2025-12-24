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

// CleanupResult 清理结果
type CleanupResult struct {
	CleanedVideoDirs   int      `json:"cleanedVideoDirs"`
	CleanedImages      int      `json:"cleanedImages"`
	CleanedVideoFiles  int      `json:"cleanedVideoFiles"`
	CleanedIndexFiles  int      `json:"cleanedIndexFiles"`
	CleanedImageFiles  int      `json:"cleanedImageFiles"`
	Errors             []string `json:"errors"`
	DryRun             bool     `json:"dryRun"`
}

// CleanupOrphanedResources 清理孤立资源（软删除超过30天的）
// dryRun: 如果为true，只返回将要清理的内容，不实际执行删除
func CleanupOrphanedResources(dryRun bool) CleanupResult {
	result := CleanupResult{
		Errors: make([]string, 0),
		DryRun: dryRun,
	}

	// 30天前的时间
	expireTime := time.Now().AddDate(0, 0, -30)

	// 1. 清理软删除超过30天的视频相关文件
	result.cleanExpiredVideoResources(expireTime, dryRun)

	// 2. 清理孤立的视频目录（数据库中不存在引用的）
	result.cleanOrphanedVideoDirs(dryRun)

	// 3. 清理孤立的图片文件
	result.cleanOrphanedImages(dryRun)

	return result
}

// cleanExpiredVideoResources 清理过期的视频资源（软删除超过30天）
func (r *CleanupResult) cleanExpiredVideoResources(expireTime time.Time, dryRun bool) {
	// 查询软删除超过30天的VideoIndexFile记录
	var expiredIndexFiles []model.VideoIndexFile
	global.Mysql.Unscoped().Where("deleted_at IS NOT NULL AND deleted_at < ?", expireTime).Find(&expiredIndexFiles)

	// 收集需要删除的目录名
	dirNamesToDelete := make(map[string]bool)
	for _, indexFile := range expiredIndexFiles {
		dirNamesToDelete[indexFile.DirName] = true
	}

	// 删除视频文件目录和OSS文件
	for dirName := range dirNamesToDelete {
		// 检查是否还有未删除的记录引用这个目录
		var activeCount int64
		global.Mysql.Model(&model.VideoIndexFile{}).Where("dir_name = ? AND deleted_at IS NULL", dirName).Count(&activeCount)
		if activeCount > 0 {
			continue // 还有活跃引用，跳过
		}

		localDir := "./upload/video/" + dirName
		if utils.IsFileExists(localDir) {
			if !dryRun {
				// 删除OSS上的文件
				deleteVideoFromOSS(localDir)
				// 删除本地目录
				if err := os.RemoveAll(localDir); err != nil {
					r.Errors = append(r.Errors, "删除视频目录失败: "+localDir+" - "+err.Error())
				} else {
					r.CleanedVideoDirs++
				}
			} else {
				r.CleanedVideoDirs++
			}
		}
	}

	// 物理删除过期的数据库记录
	if !dryRun {
		// 删除VideoIndexFile记录
		result := global.Mysql.Unscoped().Where("deleted_at IS NOT NULL AND deleted_at < ?", expireTime).Delete(&model.VideoIndexFile{})
		r.CleanedIndexFiles = int(result.RowsAffected)

		// 删除关联的VideoFile记录
		for dirName := range dirNamesToDelete {
			var activeCount int64
			global.Mysql.Model(&model.VideoIndexFile{}).Where("dir_name = ? AND deleted_at IS NULL", dirName).Count(&activeCount)
			if activeCount == 0 {
				global.Mysql.Unscoped().Where("dir_name = ?", dirName).Delete(&model.VideoFile{})
				r.CleanedVideoFiles++
			}
		}
	}
}

// cleanOrphanedVideoDirs 清理孤立的视频目录
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

		// 检查数据库中是否有引用（包括软删除的，因为可能还在30天内）
		var indexCount int64
		global.Mysql.Unscoped().Model(&model.VideoIndexFile{}).Where("dir_name = ?", dirName).Count(&indexCount)

		if indexCount == 0 {
			// 没有任何引用，可以安全删除
			localDir := filepath.Join(videoDir, dirName)
			if !dryRun {
				// 删除OSS上的文件
				deleteVideoFromOSS(localDir)
				// 删除本地目录
				if err := os.RemoveAll(localDir); err != nil {
					r.Errors = append(r.Errors, "删除孤立视频目录失败: "+localDir+" - "+err.Error())
				} else {
					r.CleanedVideoDirs++
				}
			} else {
				r.CleanedVideoDirs++
			}
		}
	}
}

// cleanOrphanedImages 清理孤立的图片文件
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

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		imageUrl := "/api/image/" + fileName

		// 检查是否被任何内容引用（包括软删除30天内的）
		// 这会检查所有 Video、Article、User、Carousel 表中的引用
		if isImageReferenced(imageUrl) {
			continue
		}

		// 如果没有任何引用，可以删除文件
		localPath := filepath.Join(imageDir, fileName)
		if !dryRun {
			// 删除OSS上的图片
			if global.Config.Storage.OssType != "local" {
				global.Storage.DeleteObject("image/" + fileName)
			}
			// 删除本地文件
			if err := os.Remove(localPath); err != nil {
				r.Errors = append(r.Errors, "删除孤立图片失败: "+localPath+" - "+err.Error())
			} else {
				r.CleanedImages++
			}
			// 删除所有关联的ImageFile记录（可能有多条，因为hash相同但不同用户上传）
			result := global.Mysql.Where("file_name = ?", fileName).Delete(&model.ImageFile{})
			r.CleanedImageFiles += int(result.RowsAffected)
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
func deleteVideoFromOSS(localDir string) {
	if global.Config.Storage.OssType == "local" {
		return
	}

	// 遍历本地目录，删除OSS上对应的文件
	filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// 构建OSS对象key
		relativePath := strings.TrimPrefix(path, "./upload/")
		relativePath = strings.ReplaceAll(relativePath, "\\", "/")
		global.Storage.DeleteObject(relativePath)
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
