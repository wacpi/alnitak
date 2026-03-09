package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/resp"
	"interastral-peace.com/alnitak/internal/service"
)

func CreatePGC(ctx *gin.Context) {
	var req dto.CreatePGCReq
	if err := ctx.ShouldBind(&req); err != nil {
		resp.FailWithMessage(ctx, "参数错误: "+err.Error())
		return
	}

	pgcID, err := service.CreatePGCContent(req)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithData(ctx, gin.H{"pgc_id": pgcID})
}

func UpdatePGC(ctx *gin.Context) {
	var req dto.UpdatePGCReq
	if err := ctx.ShouldBind(&req); err != nil {
		resp.FailWithMessage(ctx, "参数错误: "+err.Error())
		return
	}

	if err := service.UpdatePGCContent(req); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithMessage(ctx, "更新成功")
}

func DeletePGC(ctx *gin.Context) {
	pgcID := ctx.Param("pgc_id")

	pgcIDUint, err := convertToUint(pgcID)
	if err != nil {
		resp.FailWithMessage(ctx, "无效的PGC ID")
		return
	}

	if err := service.DeletePGCContent(pgcIDUint); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithMessage(ctx, "删除成功")
}

func GetPGCList(ctx *gin.Context) {
	var req dto.PGCListReq
	if err := ctx.ShouldBind(&req); err != nil {
		resp.FailWithMessage(ctx, "参数错误: "+err.Error())
		return
	}

	total, list, err := service.GetPGCContentList(req)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithData(ctx, gin.H{
		"total":     total,
		"list":      list,
		"page":      req.Page,
		"page_size": req.PageSize,
	})
}

func GetPGCDetail(ctx *gin.Context) {
	pgcID := ctx.Query("pgc_id")

	pgcIDUint, err := convertToUint(pgcID)
	if err != nil {
		resp.FailWithMessage(ctx, "无效的PGC ID")
		return
	}

	pgc, err := service.GetPGCContentDetail(pgcIDUint)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithData(ctx, gin.H{"pgc": pgc})
}

func GetPGCEpisodes(ctx *gin.Context) {
	pgcIDStr := ctx.Param("pgc_id")
	pgcIDUint, err := convertToUint(pgcIDStr)
	if err != nil {
		resp.FailWithMessage(ctx, "无效的PGC ID")
		return
	}
	var req dto.EpisodeListReq
	if err := ctx.ShouldBind(&req); err != nil {
		resp.FailWithMessage(ctx, "参数错误: "+err.Error())
		return
	}
	req.PGCID = pgcIDUint

	episodes, err := service.GetPGCEpisodeList(req.PGCID, req)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithData(ctx, gin.H{"episodes": episodes})
}

func AddPGCEpisode(ctx *gin.Context) {
	pgcID := ctx.Param("pgc_id")

	pgcIDUint, err := convertToUint(pgcID)
	if err != nil {
		resp.FailWithMessage(ctx, "无效的PGC ID")
		return
	}

	var req dto.EpisodeReq
	if err := ctx.ShouldBind(&req); err != nil {
		resp.FailWithMessage(ctx, "参数错误: "+err.Error())
		return
	}

	if err := service.AddPGCEpisode(pgcIDUint, req); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithMessage(ctx, "添加成功")
}

func DeletePGCEpisode(ctx *gin.Context) {
	pgcID := ctx.Param("pgc_id")
	episodeID := ctx.Param("id")

	pgcIDUint, err1 := convertToUint(pgcID)
	episodeIDUint, err2 := convertToUint(episodeID)
	if err1 != nil || err2 != nil {
		resp.FailWithMessage(ctx, "无效的ID")
		return
	}

	if err := service.DeletePGCEpisode(pgcIDUint, episodeIDUint); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithMessage(ctx, "删除成功")
}

func SearchPGC(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	pgcType := ctx.Query("pgc_type")
	page := ctx.Query("page")
	pageSize := ctx.Query("page_size")

	pgcTypeInt, err1 := convertToInt(pgcType)
	pageInt, err2 := convertToInt(page)
	pageSizeInt, err3 := convertToInt(pageSize)
	if err1 != nil || err2 != nil || err3 != nil {
		resp.FailWithMessage(ctx, "无效的参数")
		return
	}

	total, list, err := service.SearchPGC(keyword, pgcTypeInt, pageInt, pageSizeInt)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithData(ctx, gin.H{
		"total":     total,
		"list":      list,
		"page":      pageInt,
		"page_size": pageSizeInt,
	})
}

func GetPGCByType(ctx *gin.Context) {
	pgcType := ctx.Query("pgc_type")
	page := ctx.Query("page")
	pageSize := ctx.Query("page_size")

	pgcTypeInt, err1 := convertToInt(pgcType)
	pageInt, err2 := convertToInt(page)
	pageSizeInt, err3 := convertToInt(pageSize)
	if err1 != nil || err2 != nil || err3 != nil {
		resp.FailWithMessage(ctx, "无效的参数")
		return
	}

	total, list, err := service.GetPGCByType(pgcTypeInt, pageInt, pageSizeInt)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithData(ctx, gin.H{
		"total":     total,
		"list":      list,
		"page":      pageInt,
		"page_size": pageSizeInt,
	})
}

func GetOngoingPGC(ctx *gin.Context) {
	page := ctx.Query("page")
	pageSize := ctx.Query("page_size")

	pageInt, err1 := convertToInt(page)
	pageSizeInt, err2 := convertToInt(pageSize)
	if err1 != nil || err2 != nil {
		resp.FailWithMessage(ctx, "无效的分页参数")
		return
	}

	total, list, err := service.GetOngoingPGC(pageInt, pageSizeInt)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithData(ctx, gin.H{
		"total":     total,
		"list":      list,
		"page":      pageInt,
		"page_size": pageSizeInt,
	})
}

func GetRecommendedPGC(ctx *gin.Context) {
	limit := ctx.Query("limit")
	limitInt, err := convertToInt(limit)
	if err != nil {
		resp.FailWithMessage(ctx, "无效的limit参数")
		return
	}

	list, err := service.GetRecommendedPGC(limitInt)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithData(ctx, gin.H{"list": list})
}

func GetPGCDetailWithEpisodes(ctx *gin.Context) {
	pgcID := ctx.Query("pgc_id")

	pgcIDUint, err := convertToUint(pgcID)
	if err != nil {
		resp.FailWithMessage(ctx, "无效的PGC ID")
		return
	}

	pgc, episodes, err := service.GetPGCWithEpisodes(pgcIDUint)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.OkWithData(ctx, gin.H{
		"pgc":      pgc,
		"episodes": episodes,
	})
}

func convertToUint(s string) (uint, error) {
	var result uint
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

func convertToInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
