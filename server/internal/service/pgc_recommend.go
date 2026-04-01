package service

import (
	"errors"
	"sort"
	"strconv"

	"gorm.io/gorm"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/global"
)

type pgcLatestEpisode struct {
	ID            uint    `gorm:"column:id"`
	PGCID         uint64  `gorm:"column:pgc_id"`
	EpisodeNumber int     `gorm:"column:episode_number"`
	Title         string  `gorm:"column:title"`
	VID           uint    `gorm:"column:vid"`
	PublishTime   string  `gorm:"column:publish_time"`
	Duration      float64 `gorm:"column:duration"`
}

// playablePGCSub 构建「可播 PGC」候选子查询（每次调用一条新链，避免复用 *gorm.DB 被 Count 污染导致 Pluck 空结果）。
//
// 注意：Table + 手写 JOIN 不会自动套用 Model 的软删 Scope，而下方 Find(&[]PGCContent) 会带 deleted_at IS NULL。
// 若不在此显式排除软删行，会出现 total/pluck 命中已删数据、Find 为空 → list 始终为 []。
func playablePGCSub(pgcType int, seed uint64) *gorm.DB {
	q := global.Mysql.Table("pgc_content pc").
		Select("pc.pgc_id").
		Where("pc.deleted_at IS NULL").
		Joins("JOIN pgc_episode pe ON pe.pgc_id = pc.pgc_id AND pe.deleted_at IS NULL AND pe.status = ?", global.PGCEpisodeNormal).
		Joins("JOIN video v ON v.id = pe.vid AND v.deleted_at IS NULL AND v.status = ?", global.AUDIT_APPROVED).
		Where("pc.status = ?", global.PGCAuditApproved).
		Group("pc.pgc_id")
	if pgcType > 0 {
		q = q.Where("pc.pgc_type = ?", pgcType)
	}
	if seed > 0 {
		q = q.Where("pc.pgc_id <> ?", seed)
	}
	return q
}

// RecommendPGC 参考 B 站思路：PGC 独立召回 + 可播过滤（剧集正常且关联视频审核通过）。
//
// 返回值：
// - total: 可播候选总数（用于分页）
// - list: 该页推荐的 PGC 内容（按 ongoing/评分/新鲜度排序）
// - latestEpisode: 每个 PGC 对应的最新可播剧集信息（若无则不会出现）
func RecommendPGC(req dto.PGCRecommendReq) (total int64, list []model.PGCContent, latestEpisode map[uint64]pgcLatestEpisode, err error) {
	// ========== 参数与默认 ==========
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 50 {
		req.PageSize = 50
	}

	var seed uint64
	if req.SeedPGCID != "" {
		if seed, err = strconv.ParseUint(req.SeedPGCID, 10, 64); err != nil {
			return 0, nil, nil, errors.New("无效的seed_pgc_id")
		}
	}

	// 若未显式指定 pgc_type，尝试从 seed 推断类型
	pgcType := req.PGCType
	if pgcType <= 0 && seed > 0 {
		var seedContent model.PGCContent
		if e := global.Mysql.Select("pgc_type").
			Where("pgc_id = ? AND status = ?", seed, global.PGCAuditApproved).
			First(&seedContent).Error; e == nil {
			pgcType = seedContent.PGCType
		}
	}

	// ========== 可播候选 subquery（按 B 站：先召回再过滤，这里用 SQL 直接过滤） ==========
	//
	// 可播定义（MVP）：
	// - PGC 审核通过
	// - 至少存在 1 个剧集（pgc_episode.status=normal）
	// - 且该剧集关联的视频审核通过（video.status=AUDIT_APPROVED）
	//
	// 注意：Count 与 Pluck 必须各用独立子查询链；同一 *gorm.DB 先作为 Table("(?) as t", sub) 子查询再链式 Order/Pluck
	// 会污染 Statement，出现 total>0 但 Pluck 为空（与线上现象一致）。

	// total（distinct pgc_id）
	if e := global.Mysql.Table("(?) as t", playablePGCSub(pgcType, seed)).Count(&total).Error; e != nil {
		return 0, nil, nil, errors.New("查询失败")
	}
	if total == 0 {
		return 0, []model.PGCContent{}, map[uint64]pgcLatestEpisode{}, nil
	}

	// ========== 分页取 pgc_id，再回表取内容（避免 only_full_group_by 问题） ==========
	start := (req.Page - 1) * req.PageSize
	var pgcIDs []uint64
	if e := playablePGCSub(pgcType, seed).
		Order("pc.is_ongoing DESC, pc.rating DESC, pc.created_at DESC").
		Limit(req.PageSize).
		Offset(start).
		Pluck("pc.pgc_id", &pgcIDs).Error; e != nil {
		return 0, nil, nil, errors.New("查询失败")
	}
	if len(pgcIDs) == 0 {
		return total, []model.PGCContent{}, map[uint64]pgcLatestEpisode{}, nil
	}

	var contents []model.PGCContent
	if e := global.Mysql.Where("pgc_id IN ?", pgcIDs).
		Find(&contents).Error; e != nil {
		return 0, nil, nil, errors.New("查询失败")
	}

	// 按 pgcIDs 维持排序
	order := make(map[uint64]int, len(pgcIDs))
	for i, id := range pgcIDs {
		order[id] = i
	}
	sort.Slice(contents, func(i, j int) bool {
		return order[contents[i].PGCID] < order[contents[j].PGCID]
	})
	list = contents

	// ========== 批量取“最新可播剧集”（用于卡片展示 index_show/new_ep） ==========
	var eps []pgcLatestEpisode
	if e := global.Mysql.Table("pgc_episode pe").
		Select("pe.id, pe.pgc_id, pe.episode_number, pe.title, pe.vid, pe.publish_time, pe.duration").
		Joins("JOIN video v ON v.id = pe.vid AND v.status = ?", global.AUDIT_APPROVED).
		Where("pe.pgc_id IN ?", pgcIDs).
		Where("pe.status = ?", global.PGCEpisodeNormal).
		Order("pe.pgc_id ASC, pe.episode_number DESC, pe.id DESC").
		Scan(&eps).Error; e != nil {
		// 不影响主流程：latestEpisode 为空即可
		latestEpisode = map[uint64]pgcLatestEpisode{}
		return total, list, latestEpisode, nil
	}
	latestEpisode = make(map[uint64]pgcLatestEpisode, len(pgcIDs))
	for _, ep := range eps {
		if ep.PGCID == 0 {
			continue
		}
		if _, ok := latestEpisode[ep.PGCID]; ok {
			continue
		}
		latestEpisode[ep.PGCID] = ep
	}

	return total, list, latestEpisode, nil
}

// RecommendPGCByVideo 按当前播放视频关联的 PGC 做同类推荐。
// 规则：先由 vid 找到其所在 pgc_id，再按该 season 作为 seed 召回推荐列表。
func RecommendPGCByVideo(vid uint, page, pageSize int) (total int64, list []model.PGCContent, latestEpisode map[uint64]pgcLatestEpisode, err error) {
	if vid == 0 {
		return 0, []model.PGCContent{}, map[uint64]pgcLatestEpisode{}, nil
	}

	var ep model.PGCEpisode
	if e := global.Mysql.Select("pgc_id").
		Where("vid = ? AND status = ?", vid, global.PGCEpisodeNormal).
		Order("id DESC").
		First(&ep).Error; e != nil || ep.PGCID == 0 {
		// 当前视频未绑定 PGC，返回空结果，前端可按需兜底
		return 0, []model.PGCContent{}, map[uint64]pgcLatestEpisode{}, nil
	}

	req := dto.PGCRecommendReq{
		Page:      page,
		PageSize:  pageSize,
		SeedPGCID: strconv.FormatUint(ep.PGCID, 10),
		Scene:     "watch",
	}
	return RecommendPGC(req)
}
