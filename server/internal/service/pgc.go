package service

import (
	"errors"

	"gorm.io/gorm"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

func CreatePGCContent(req dto.CreatePGCReq) (uint, error) {
	if req.Title == "" {
		return 0, errors.New("标题不能为空")
	}

	if len(req.Title) > 255 {
		return 0, errors.New("标题长度不能超过255字符")
	}

	if req.Cover == "" {
		return 0, errors.New("封面不能为空")
	}

	if req.PGCType <= 0 {
		return 0, errors.New("PGC类型不能为空")
	}

	validTypes := map[int]bool{
		global.PGCTypeBangumi:  true,
		global.PGCTypeDocument: true,
		global.PGCTypeMovie:    true,
		global.PGCTypeTVSeries: true,
	}

	if !validTypes[req.PGCType] {
		return 0, errors.New("无效的PGC类型")
	}

	if req.Rating < 0 || req.Rating > 10 {
		return 0, errors.New("评分必须在0-10之间")
	}

	if len(req.Episodes) == 0 {
		utils.ErrorLog("创建PGC内容失败", "pgc", "至少需要一集")
		return 0, errors.New("至少需要一集")
	}

	vids := make([]uint, 0, len(req.Episodes))
	for _, episode := range req.Episodes {
		vids = append(vids, episode.VID)
	}

	var count int64
	if err := global.Mysql.Model(&model.Video{}).
		Where("id IN ?", vids).
		Count(&count).Error; err != nil {
		utils.ErrorLog("校验视频存在失败", "pgc", err.Error())
		return 0, errors.New("校验视频失败")
	}

	if count != int64(len(vids)) {
		return 0, errors.New("部分视频不存在")
	}

	episodeMap := make(map[int]bool)
	for _, episode := range req.Episodes {
		if episodeMap[episode.EpisodeNumber] {
			return 0, errors.New("剧集序号重复")
		}
		episodeMap[episode.EpisodeNumber] = true
	}

	pgcID := uint(global.SnowflakeNode.Generate())

	tx := global.Mysql.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			utils.ErrorLog("创建PGC内容panic", "pgc", "")
		}
	}()

	pgcContent := &model.PGCContent{
		PGCID:           pgcID,
		PGCType:         req.PGCType,
		Title:           req.Title,
		Cover:           req.Cover,
		Desc:            req.Desc,
		Year:            req.Year,
		Area:            req.Area,
		Rating:          req.Rating,
		IsOngoing:       req.IsOngoing,
		TotalEpisodes:   len(req.Episodes),
		CurrentEpisodes: len(req.Episodes),
		Status:          global.PGCAuditApproved,
	}

	if err := tx.Create(pgcContent).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("创建PGC内容失败", "pgc", err.Error())
		return 0, errors.New("创建PGC内容失败")
	}

	for _, episode := range req.Episodes {
		pgcEpisode := &model.PGCEpisode{
			PGCID:         pgcID,
			EpisodeNumber: episode.EpisodeNumber,
			Title:         episode.Title,
			VID:           episode.VID,
			Duration:      episode.Duration,
			Status:        global.PGCEpisodeNormal,
			PublishTime:   episode.PublishTime,
		}

		if err := tx.Create(pgcEpisode).Error; err != nil {
			tx.Rollback()
			utils.ErrorLog("创建PGC剧集失败", "pgc", err.Error())
			return 0, errors.New("创建剧集失败")
		}
	}

	if err := tx.Commit().Error; err != nil {
		utils.ErrorLog("提交事务失败", "pgc", err.Error())
		return 0, errors.New("创建失败")
	}

	utils.InfoLog("创建PGC内容成功", "pgc")

	return pgcID, nil
}

func UpdatePGCContent(req dto.UpdatePGCReq) error {
	if req.PGCID == 0 {
		return errors.New("PGC ID不能为空")
	}

	if req.Title != "" && len(req.Title) > 255 {
		return errors.New("标题长度不能超过255字符")
	}

	if req.Rating < 0 || req.Rating > 10 {
		return errors.New("评分必须在0-10之间")
	}

	var pgcContent model.PGCContent
	if err := global.Mysql.Where("pgc_id = ?", req.PGCID).First(&pgcContent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.ErrorLog("PGC内容不存在", "pgc", "")
			return errors.New("PGC内容不存在")
		}
		utils.ErrorLog("查询PGC内容失败", "pgc", err.Error())
		return errors.New("查询失败")
	}

	updates := make(map[string]interface{})

	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Cover != "" {
		updates["cover"] = req.Cover
	}
	if req.Desc != "" {
		updates["desc"] = req.Desc
	}
	if req.Year > 0 {
		updates["year"] = req.Year
	}
	if req.Area != "" {
		updates["area"] = req.Area
	}
	if req.Rating >= 0 {
		updates["rating"] = req.Rating
	}

	if len(updates) == 0 {
		return nil
	}

	if err := global.Mysql.Model(&model.PGCContent{}).
		Where("pgc_id = ?", req.PGCID).
		Updates(updates).Error; err != nil {
		utils.ErrorLog("更新PGC内容失败", "pgc", err.Error())
		return errors.New("更新失败")
	}

	utils.InfoLog("更新PGC内容成功", "pgc")

	return nil
}

func DeletePGCContent(pgcID uint) error {
	if pgcID == 0 {
		utils.ErrorLog("删除PGC内容参数错误", "pgc", "pgcID不能为空")
		return errors.New("PGC ID不能为空")
	}

	var count int64
	if err := global.Mysql.Model(&model.PGCContent{}).
		Where("pgc_id = ?", pgcID).
		Count(&count).Error; err != nil {
		utils.ErrorLog("查询PGC内容失败", "pgc", err.Error())
		return errors.New("查询失败")
	}

	if count == 0 {
		utils.ErrorLog("PGC内容不存在", "pgc", "")
		return errors.New("PGC内容不存在")
	}

	tx := global.Mysql.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			utils.ErrorLog("删除PGC内容panic", "pgc", "")
		}
	}()

	if err := tx.Delete(&model.PGCContent{}, "pgc_id = ?", pgcID).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("删除PGC内容失败", "pgc", err.Error())
		return errors.New("删除失败")
	}

	if err := tx.Delete(&model.PGCEpisode{}, "pgc_id = ?", pgcID).Error; err != nil {
		tx.Rollback()
		utils.ErrorLog("删除PGC剧集失败", "pgc", err.Error())
		return errors.New("删除剧集失败")
	}

	if err := tx.Commit().Error; err != nil {
		utils.ErrorLog("提交事务失败", "pgc", err.Error())
		return errors.New("删除失败")
	}

	utils.InfoLog("删除PGC内容成功", "pgc")

	return nil
}

func GetPGCContentList(req dto.PGCListReq) (total int64, list []model.PGCContent, err error) {
	query := global.Mysql.Model(&model.PGCContent{})

	if req.PGCType > 0 {
		query = query.Where("pgc_type = ?", req.PGCType)
	}
	if req.Status >= 0 {
		query = query.Where("status = ?", req.Status)
	}
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		query = query.Where("title LIKE ? OR desc LIKE ?", keyword, keyword)
	}
	if req.Year > 0 {
		query = query.Where("year = ?", req.Year)
	}
	if req.Area != "" {
		query = query.Where("area = ?", req.Area)
	}
	if req.IsOngoing {
		query = query.Where("is_ongoing = ?", req.IsOngoing)
	}

	if err := query.Count(&total).Error; err != nil {
		utils.ErrorLog("查询PGC内容总数失败", "pgc", err.Error())
		return 0, nil, errors.New("查询失败")
	}

	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("created_at DESC").
		Limit(req.PageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		utils.ErrorLog("查询PGC内容列表失败", "pgc", err.Error())
		return 0, nil, errors.New("查询失败")
	}

	return
}

func GetPGCContentDetail(pgcID uint) (*model.PGCContent, error) {
	if pgcID == 0 {
		return nil, errors.New("PGC ID不能为空")
	}

	var pgcContent model.PGCContent
	if err := global.Mysql.Where("pgc_id = ?", pgcID).First(&pgcContent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.ErrorLog("PGC内容不存在", "pgc", "")
			return nil, errors.New("PGC内容不存在")
		}
		utils.ErrorLog("查询PGC内容详情失败", "pgc", err.Error())
		return nil, errors.New("查询失败")
	}

	return &pgcContent, nil
}

func GetPGCEpisodeList(pgcID uint, req dto.EpisodeListReq) ([]model.PGCEpisode, error) {
	if pgcID == 0 {
		utils.ErrorLog("获取PGC剧集列表参数错误", "pgc", "pgcID不能为空")
		return nil, errors.New("PGC ID不能为空")
	}

	var count int64
	if err := global.Mysql.Model(&model.PGCContent{}).
		Where("pgc_id = ?", pgcID).
		Count(&count).Error; err != nil {
		utils.ErrorLog("查询PGC内容失败", "pgc", err.Error())
		return nil, errors.New("查询失败")
	}

	if count == 0 {
		utils.ErrorLog("PGC内容不存在", "pgc", "")
		return nil, errors.New("PGC内容不存在")
	}

	var episodes []model.PGCEpisode
	query := global.Mysql.Model(&model.PGCEpisode{}).Where("pgc_id = ?", pgcID)

	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("episode_number ASC").
		Limit(req.PageSize).
		Offset(offset).
		Find(&episodes).Error; err != nil {
		utils.ErrorLog("查询PGC剧集列表失败", "pgc", err.Error())
		return nil, errors.New("查询失败")
	}

	return episodes, nil
}

func AddPGCEpisode(pgcID uint, episode dto.EpisodeReq) error {
	if pgcID == 0 {
		utils.ErrorLog("添加PGC剧集参数错误", "pgc", "pgcID不能为空")
		return errors.New("PGC ID不能为空")
	}

	var pgcContent model.PGCContent
	if err := global.Mysql.Where("pgc_id = ?", pgcID).First(&pgcContent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.ErrorLog("PGC内容不存在", "pgc", "")
			return errors.New("PGC内容不存在")
		}
		utils.ErrorLog("查询PGC内容失败", "pgc", err.Error())
		return errors.New("查询失败")
	}

	var count int64
	if err := global.Mysql.Model(&model.Video{}).
		Where("id = ?", episode.VID).
		Count(&count).Error; err != nil {
		utils.ErrorLog("查询视频失败", "pgc", err.Error())
		return errors.New("查询视频失败")
	}

	if count == 0 {
		return errors.New("视频不存在")
	}

	var episodeCount int64
	if err := global.Mysql.Model(&model.PGCEpisode{}).
		Where("pgc_id = ? AND episode_number = ?", pgcID, episode.EpisodeNumber).
		Count(&episodeCount).Error; err != nil {
		utils.ErrorLog("查询PGC剧集失败", "pgc", err.Error())
		return errors.New("查询失败")
	}

	if episodeCount > 0 {
		utils.ErrorLog("剧集序号已存在", "pgc", "")
		return errors.New("剧集序号已存在")
	}

	pgcEpisode := &model.PGCEpisode{
		PGCID:         pgcID,
		EpisodeNumber: episode.EpisodeNumber,
		Title:         episode.Title,
		VID:           episode.VID,
		Duration:      episode.Duration,
		Status:        global.PGCEpisodeNormal,
		PublishTime:   episode.PublishTime,
	}

	if err := global.Mysql.Create(pgcEpisode).Error; err != nil {
		utils.ErrorLog("创建PGC剧集失败", "pgc", err.Error())
		return errors.New("创建失败")
	}

	if err := global.Mysql.Model(&model.PGCContent{}).
		Where("pgc_id = ?", pgcID).
		UpdateColumn("current_episodes", gorm.Expr("current_episodes + 1")).Error; err != nil {
		utils.ErrorLog("更新PGC内容集数失败", "pgc", err.Error())
	}

	utils.InfoLog("添加PGC剧集成功", "pgc")

	return nil
}

func DeletePGCEpisode(pgcID uint, episodeID uint) error {
	if pgcID == 0 || episodeID == 0 {
		utils.ErrorLog("删除PGC剧集参数错误", "pgc", "参数不能为空")
		return errors.New("参数不能为空")
	}

	var count int64
	if err := global.Mysql.Model(&model.PGCEpisode{}).
		Where("id = ? AND pgc_id = ?", episodeID, pgcID).
		Count(&count).Error; err != nil {
		utils.ErrorLog("查询PGC剧集失败", "pgc", err.Error())
		return errors.New("查询失败")
	}

	if count == 0 {
		utils.ErrorLog("PGC剧集不存在", "pgc", "")
		return errors.New("剧集不存在")
	}

	if err := global.Mysql.Delete(&model.PGCEpisode{}, "id = ? AND pgc_id = ?", episodeID, pgcID).Error; err != nil {
		utils.ErrorLog("删除PGC剧集失败", "pgc", err.Error())
		return errors.New("删除失败")
	}

	if err := global.Mysql.Model(&model.PGCContent{}).
		Where("pgc_id = ?", pgcID).
		UpdateColumn("current_episodes", gorm.Expr("current_episodes - 1")).Error; err != nil {
		utils.ErrorLog("更新PGC内容集数失败", "pgc", err.Error())
	}

	utils.InfoLog("删除PGC剧集成功", "pgc")

	return nil
}

func SearchPGC(keyword string, pgcType int, page, pageSize int) (total int64, list []model.PGCContent, err error) {
	query := global.Mysql.Model(&model.PGCContent{})

	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR desc LIKE ?", likeKeyword, likeKeyword)
	}

	if pgcType > 0 {
		query = query.Where("pgc_type = ?", pgcType)
	}

	query = query.Where("status = ?", global.PGCAuditApproved)

	if err := query.Count(&total).Error; err != nil {
		utils.ErrorLog("搜索PGC内容失败", "pgc", err.Error())
		return 0, nil, errors.New("搜索失败")
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		utils.ErrorLog("搜索PGC内容失败", "pgc", err.Error())
		return 0, nil, errors.New("搜索失败")
	}

	return
}

func GetPGCByType(pgcType int, page, pageSize int) (total int64, list []model.PGCContent, err error) {
	if pgcType <= 0 {
		utils.ErrorLog("获取PGC类型列表参数错误", "pgc", "pgcType不能为空")
		return 0, nil, errors.New("PGC类型不能为空")
	}

	query := global.Mysql.Model(&model.PGCContent{}).
		Where("pgc_type = ? AND status = ?", pgcType, global.PGCAuditApproved)

	if err := query.Count(&total).Error; err != nil {
		utils.ErrorLog("获取PGC类型总数失败", "pgc", err.Error())
		return 0, nil, errors.New("查询失败")
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		utils.ErrorLog("获取PGC类型列表失败", "pgc", err.Error())
		return 0, nil, errors.New("查询失败")
	}

	return
}

func GetOngoingPGC(page, pageSize int) (total int64, list []model.PGCContent, err error) {
	query := global.Mysql.Model(&model.PGCContent{}).
		Where("is_ongoing = ? AND status = ?", true, global.PGCAuditApproved)

	if err := query.Count(&total).Error; err != nil {
		utils.ErrorLog("获取连载PGC失败", "pgc", err.Error())
		return 0, nil, errors.New("查询失败")
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		utils.ErrorLog("获取连载PGC失败", "pgc", err.Error())
		return 0, nil, errors.New("查询失败")
	}

	return
}

func GetRecommendedPGC(limit int) ([]model.PGCContent, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	var list []model.PGCContent
	if err := global.Mysql.Model(&model.PGCContent{}).
		Where("status = ?", global.PGCAuditApproved).
		Order("created_at DESC").
		Limit(limit).
		Find(&list).Error; err != nil {
		utils.ErrorLog("获取推荐PGC失败", "pgc", err.Error())
		return nil, errors.New("查询失败")
	}

	return list, nil
}

func GetPGCWithEpisodes(pgcID uint) (*model.PGCContent, []model.PGCEpisode, error) {
	if pgcID == 0 {
		return nil, nil, errors.New("PGC ID不能为空")
	}

	var pgcContent model.PGCContent
	if err := global.Mysql.Where("pgc_id = ? AND status = ?", pgcID, global.PGCAuditApproved).
		First(&pgcContent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.ErrorLog("PGC内容不存在", "pgc", "")
			return nil, nil, errors.New("PGC内容不存在")
		}
		utils.ErrorLog("查询PGC内容失败", "pgc", err.Error())
		return nil, nil, errors.New("查询失败")
	}

	var episodes []model.PGCEpisode
	if err := global.Mysql.Model(&model.PGCEpisode{}).
		Where("pgc_id = ? AND status = ?", pgcID, global.PGCEpisodeNormal).
		Order("episode_number ASC").
		Find(&episodes).Error; err != nil {
		utils.ErrorLog("查询PGC剧集失败", "pgc", err.Error())
		return &pgcContent, nil, errors.New("查询失败")
	}

	return &pgcContent, episodes, nil
}
