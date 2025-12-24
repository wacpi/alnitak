package model

import "gorm.io/gorm"

type ImageFile struct {
	gorm.Model
	Uid      uint   `gorm:"comment:用户ID;not null;index"`
	FileName string `gorm:"type:varchar(100);comment:文件名;index"`
	Hash     string `gorm:"type:varchar(64);comment:文件hash;index"`
}

func (table *ImageFile) TableName() string {
	return "image_file"
}
