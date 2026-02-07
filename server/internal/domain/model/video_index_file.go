package model

import "gorm.io/gorm"

// VideoIndexFile 视频索引文件（B站风格：音视频分离 + SegmentBase）
type VideoIndexFile struct {
	gorm.Model
	ResourceID uint   `gorm:"index;comment:视频资源ID;"`
	Quality    string `gorm:"type:varchar(30);comment:视频质量;"`
	DirName    string `gorm:"type:varchar(50);comment:目录名称;"`

	// 视频流信息
	VideoFile       string  `gorm:"type:varchar(100);comment:视频文件名;"`
	VideoBandwidth  int     `gorm:"comment:视频码率(bps);"`
	VideoCodec      string  `gorm:"type:varchar(50);comment:视频编解码器;"`
	Width           int     `gorm:"comment:视频宽度;"`
	Height          int     `gorm:"comment:视频高度;"`
	FrameRate       float64 `gorm:"type:decimal(10,3);comment:帧率;"`
	VideoInitRange  string  `gorm:"type:varchar(30);comment:视频初始化字节范围;"`  // 如 "0-927"
	VideoIndexRange string  `gorm:"type:varchar(30);comment:视频索引字节范围;"`   // 如 "928-2051"

	// 音频流信息
	AudioFile       string `gorm:"type:varchar(100);comment:音频文件名;"`
	AudioBandwidth  int    `gorm:"comment:音频码率(bps);"`
	AudioCodec      string `gorm:"type:varchar(50);comment:音频编解码器;"`
	AudioSampleRate int    `gorm:"comment:音频采样率;"`
	AudioInitRange  string `gorm:"type:varchar(30);comment:音频初始化字节范围;"`
	AudioIndexRange string `gorm:"type:varchar(30);comment:音频索引字节范围;"`

	// 通用信息
	TotalDuration float64 `gorm:"type:decimal(10,3);comment:视频总时长(秒);"`

	// 兼容旧数据（切片模式）
	SegmentDuration     float64 `gorm:"type:decimal(10,3);comment:标准分片时长(秒);"`
	SegmentCount        int     `gorm:"comment:分片总数;"`
	LastSegmentDuration float64 `gorm:"type:decimal(10,3);comment:最后分片时长(秒);"`
	InitFile            string  `gorm:"type:varchar(50);comment:初始化文件名;"`
	Bandwidth           int     `gorm:"comment:码率(bps);"`
	Codec               string  `gorm:"type:varchar(50);comment:编解码器;"`
	Content             string  `gorm:"type:text;comment:文件内容(旧);"`
}

func (table *VideoIndexFile) TableName() string {
	return "video_index_file"
}

// IsSegmentBase 判断是否使用 SegmentBase 模式（B站风格）
func (v *VideoIndexFile) IsSegmentBase() bool {
	return v.VideoFile != "" && v.VideoInitRange != ""
}

// IsSegmentList 判断是否使用切片模式
func (v *VideoIndexFile) IsSegmentList() bool {
	return v.SegmentCount > 0
}
