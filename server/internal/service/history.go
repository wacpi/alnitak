package service

import (
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

func AddHistory(ctx *gin.Context, historyReq dto.HistoryReq) error {
	userId := ctx.GetUint("userId")
	if historyReq.Part == 0 {
		historyReq.Part = 1
	}

	history, err := FindHistoryByPart(historyReq.Vid, userId, historyReq.Part)
	if err != nil && err != gorm.ErrRecordNotFound {
		utils.ErrorLog("保存历史记录失败", "history", err.Error())
		return errors.New("保存失败")
	}

	if history.ID == 0 {
		if err := global.Mysql.Create(&model.History{
			Uid:      userId,
			Vid:      historyReq.Vid,
			Time:     historyReq.Time,
			Part:     historyReq.Part,
			Duration: historyReq.Duration,
		}).Error; err != nil {
			utils.ErrorLog("保存历史记录失败", "history", err.Error())
			return errors.New("保存失败")
		}
	} else {
		history.Time = historyReq.Time
		history.Part = historyReq.Part
		history.Duration = historyReq.Duration
		if err := global.Mysql.Save(&history).Error; err != nil {
			utils.ErrorLog("保存历史记录失败", "history", err.Error())
			return errors.New("保存失败")
		}
	}

	return nil
}

func GetHistoryList(ctx *gin.Context, page, pageSize int) (videos []vo.HistoryVideoResp, err error) {
	userId := ctx.GetUint("userId")
	subQuery := global.Mysql.Model(&model.History{}).Where("uid = ?", userId).Select(vo.HISTORY_SUBQUERY_FIELD).Group("vid")
	if err := global.Mysql.Model(&model.History{}).Select(vo.HISTORY_VIDEO_FIELD).
		Joins("LEFT JOIN `video` ON `video`.id = `history`.vid").
		Joins("INNER JOIN (?) latest on `history`.vid = latest.vid and `history`.updated_at = latest.latest_updated_at", subQuery).
		Where("`history`.uid = ? and video.deleted_at is null and video.`status` = ?", userId, global.AUDIT_APPROVED).
		Order("`history`.`updated_at` desc").Limit(pageSize).Offset((page - 1) * pageSize).
		Find(&videos).Error; err != nil {
		utils.ErrorLog("获取历史记录视频失败", "history", err.Error())
		return videos, errors.New("获取失败")
	}

	enrichHistoryPGCMeta(videos)
	return
}

type historyPGCRow struct {
	Vid           uint   `gorm:"column:vid"`
	EpID          uint   `gorm:"column:ep_id"`
	EpisodeNumber int    `gorm:"column:episode_number"`
	EpisodeTitle  string `gorm:"column:episode_title"`
	PGCTitle      string `gorm:"column:pgc_title"`
}

// enrichHistoryPGCMeta 为 PGC 绑定视频补充系列标题、剧集信息与 ep_id（与播放页 ep 路由对齐）。
func enrichHistoryPGCMeta(videos []vo.HistoryVideoResp) {
	var vids []uint
	for _, v := range videos {
		if v.PGCAttached {
			vids = append(vids, v.ID)
		}
	}
	if len(vids) == 0 {
		return
	}

	var rows []historyPGCRow
	if err := global.Mysql.Table("pgc_episode pe").
		Select("pe.vid, pe.id AS ep_id, pe.episode_number, pe.title AS episode_title, pc.title AS pgc_title").
		Joins("JOIN pgc_content pc ON pc.pgc_id = pe.pgc_id AND pc.deleted_at IS NULL").
		Where("pe.vid IN ? AND pe.deleted_at IS NULL", vids).
		Order("pe.id DESC").
		Scan(&rows).Error; err != nil {
		return
	}

	mergeHistoryPGCRowsIntoVideos(videos, rows)
}

// mergeHistoryPGCRowsIntoVideos 将剧集查询结果合并进历史列表（按 vid 去重：同 vid 多行时保留 ORDER 中先出现的一条，通常即 id 最大）。
func mergeHistoryPGCRowsIntoVideos(videos []vo.HistoryVideoResp, rows []historyPGCRow) {
	byVid := make(map[uint]historyPGCRow, len(rows))
	for _, r := range rows {
		if _, ok := byVid[r.Vid]; !ok {
			byVid[r.Vid] = r
		}
	}
	for i := range videos {
		if !videos[i].PGCAttached {
			continue
		}
		r, ok := byVid[videos[i].ID]
		if !ok {
			continue
		}
		videos[i].EpID = r.EpID
		videos[i].EpisodeNumber = r.EpisodeNumber
		videos[i].EpisodeTitle = r.EpisodeTitle
		videos[i].PGCTitle = r.PGCTitle
	}
}

func GetHistoryProgress(ctx *gin.Context, videoId, part uint) (progress float64, realPart uint, err error) {
	userId := ctx.GetUint("userId")
	var history model.History
	if part == 0 {
		history, err = FindLatestHistory(videoId, userId)
	} else {
		history, err = FindHistoryByPart(videoId, userId, part)
	}
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 无历史记录，返回进度0，不视为错误
			return 0, 0, nil
		}
		utils.ErrorLog("获取历史记录进度失败", "history", err.Error())
		return 0, 0, errors.New("获取失败")
	}
	return history.Time, history.Part, nil
}

func FindLatestHistory(videoId, userId uint) (history model.History, err error) {
	if err = global.Mysql.Where("vid = ? and uid= ?", videoId, userId).Order("updated_at desc").First(&history).Error; err != nil {
		return
	}

	return
}

func FindHistoryByPart(videoId, userId, part uint) (history model.History, err error) {
	if err = global.Mysql.Where("vid = ? and uid= ? and part = ?", videoId, userId, part).First(&history).Error; err != nil {
		return
	}

	return
}
