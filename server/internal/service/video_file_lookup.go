package service

import (
	"gorm.io/gorm"
	"interastral-peace.com/alnitak/internal/domain/model"
)

// findVideoFileByDirName 用于兼容旧数据：Resource.FileID 可能为 0，需要通过 dirName 找到对应的 VideoFile。
// 入参 db 用于确保调用者事务语义（如 updateVideoFileStatus 内部传入事务 db）。
func findVideoFileByDirName(db *gorm.DB, dirName string) (*model.VideoFile, error) {
	var vf model.VideoFile
	if err := db.Where("dir_name = ?", dirName).First(&vf).Error; err != nil {
		return nil, err
	}
	return &vf, nil
}

