package initialize

import (
	"fmt"

	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

func InitTables() {
	global.Mysql.AutoMigrate(&model.User{})           // 用户表
	global.Mysql.AutoMigrate(&model.UserBan{})        // 用户封禁表
	global.Mysql.AutoMigrate(&model.Role{})           // 角色表
	global.Mysql.AutoMigrate(&model.Menu{})           // 菜单表
	global.Mysql.AutoMigrate(&model.Api{})            // Api表
	global.Mysql.AutoMigrate(&model.CasbinRule{})     // casbin规则表
	global.Mysql.AutoMigrate(&model.Operate{})        // 操作日志表
	global.Mysql.AutoMigrate(&model.Partition{})      // 分区表
	global.Mysql.AutoMigrate(&model.Video{})          // 视频表
	global.Mysql.AutoMigrate(&model.VideoFile{})      // 视频文件表（全局去重）
	global.Mysql.AutoMigrate(&model.VideoFileRef{})   // 视频文件引用表
	global.Mysql.AutoMigrate(&model.Resource{})       // 视频资源表
	global.Mysql.AutoMigrate(&model.VideoIndexFile{}) // 视频播放索引文件表
	global.Mysql.AutoMigrate(&model.Review{})         // 视频审核表
	global.Mysql.AutoMigrate(&model.Comment{})        // 评论回复表
	global.Mysql.AutoMigrate(&model.LikeVideo{})      // 视频点赞表
	global.Mysql.AutoMigrate(&model.LikeArticle{})    // 文章点赞表
	global.Mysql.AutoMigrate(&model.CollectVideo{})   // 视频收藏表
	global.Mysql.AutoMigrate(&model.CollectArticle{}) // 文章收藏表
	global.Mysql.AutoMigrate(&model.Collection{})     // 收藏夹表
	global.Mysql.AutoMigrate(&model.Relation{})       // 关系表
	global.Mysql.AutoMigrate(&model.Danmaku{})        // 弹幕表
	global.Mysql.AutoMigrate(&model.History{})        // 历史记录表
	global.Mysql.AutoMigrate(&model.Announce{})       // 公告表
	global.Mysql.AutoMigrate(&model.LikeMessage{})    // 点赞消息表
	global.Mysql.AutoMigrate(&model.AtMessage{})      // @消息表
	global.Mysql.AutoMigrate(&model.ReplyMessage{})   // 回复消息表
	global.Mysql.AutoMigrate(&model.Whisper{})           // 私信消息表
	global.Mysql.AutoMigrate(&model.MessageReadStatus{}) // 公告/点赞/回复/@ 已读进度表
	global.Mysql.AutoMigrate(&model.Carousel{})          // 轮播图表
	global.Mysql.AutoMigrate(&model.Article{})        // 文章表
	global.Mysql.AutoMigrate(&model.ImageFile{})      // 图片文件表
	global.Mysql.AutoMigrate(&model.Playlist{})       // 合集表
	global.Mysql.AutoMigrate(&model.PlaylistVideo{})  // 合集视频关联表
	global.Mysql.AutoMigrate(&model.PGCContent{})     // PGC内容表
	global.Mysql.AutoMigrate(&model.PGCEpisode{})     // PGC剧集表

	// 补填已有记录的空 ShortID
	backfillShortIDs()
}

// backfillShortIDs 为已有的空 short_id 记录补填短ID
func backfillShortIDs() {
	backfillTable := func(tableName string) {
		var records []struct {
			ID uint
		}
		global.Mysql.Table(tableName).Where("short_id = '' OR short_id IS NULL").Select("id").Find(&records)
		if len(records) == 0 {
			return
		}
		utils.InfoLog(fmt.Sprintf("【补填ShortID】%s 共%d条空记录", tableName, len(records)), "init")
		for _, r := range records {
			shortID := utils.EncodeUint64ToShortID(uint64(global.SnowflakeNode.Generate()))
			global.Mysql.Table(tableName).Where("id = ?", r.ID).Update("short_id", shortID)
		}
		utils.InfoLog(fmt.Sprintf("【补填ShortID】%s 完成", tableName), "init")
	}

	backfillTable("video")
	backfillTable("resource")
	backfillTable("article")
}
