package service

import (
	"strings"

	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/global"
)

// SyncVideoTagsFromCSV 将逗号分隔的标签同步到 tag + video_tag；并回写 video.tags 便于 LIKE 搜索。
func SyncVideoTagsFromCSV(videoID uint, tagsCSV string) {
	tagsCSV = strings.TrimSpace(tagsCSV)
	names := splitTagNames(tagsCSV)

	// 回写冗余列（搜索兼容）
	global.Mysql.Model(&model.Video{}).Where("id = ?", videoID).Update("tags", tagsCSV)

	global.Mysql.Where("video_id = ?", videoID).Delete(&model.VideoTag{})

	for _, name := range names {
		if name == "" {
			continue
		}
		var t model.Tag
		if err := global.Mysql.Where("name = ?", name).FirstOrCreate(&t, model.Tag{Name: name}).Error; err != nil || t.ID == 0 {
			continue
		}
		global.Mysql.Create(&model.VideoTag{VideoID: videoID, TagID: t.ID})
	}
}

// LoadVideoTagNames 优先读多对多关联；若无关联行则回退 video.tags 冗余 CSV。
func LoadVideoTagNames(videoID uint) []string {
	var ids []uint
	global.Mysql.Model(&model.VideoTag{}).Where("video_id = ?", videoID).Pluck("tag_id", &ids)
	if len(ids) > 0 {
		var names []string
		global.Mysql.Model(&model.Tag{}).Where("id IN ?", ids).Order("id asc").Pluck("name", &names)
		return names
	}
	var v model.Video
	global.Mysql.Select("tags").Where("id = ?", videoID).First(&v)
	return splitTagNames(v.Tags)
}

func splitTagNames(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
