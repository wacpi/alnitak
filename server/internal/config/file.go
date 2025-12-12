package config

type File struct {
	MaxImgSize       int64    `mapstructure:"max_img_size" json:"max_img_size" yaml:"max_img_size"`
	MaxVideoSize     int64    `mapstructure:"max_video_size" json:"max_video_size" yaml:"max_video_size"`
	AllowedVideoExts []string `mapstructure:"allowed_video_exts" json:"allowed_video_exts" yaml:"allowed_video_exts"`
	AllowedImgExts   []string `mapstructure:"allowed_img_exts" json:"allowed_img_exts" yaml:"allowed_img_exts"`
}
