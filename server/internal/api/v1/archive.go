package api

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/resp"
	"interastral-peace.com/alnitak/internal/service"
)

// 通过视频ID获取点赞收藏数据
func GetVideoArchiveStat(ctx *gin.Context) {
	raw := ctx.Query("vid")
	vid, err := service.ParseVideoID(raw)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}
	if vid == 0 {
		resp.FailWithMessage(ctx, "参数有误")
		return
	}

	stat, err := service.GetVideoArchiveStat(ctx, vid)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回给前端
	resp.OkWithData(ctx, gin.H{"stat": stat})
}

// 通过文章ID获取点赞收藏数据
func GetArticleArchiveStat(ctx *gin.Context) {
	raw := ctx.Query("aid")
	aid, err := service.ParseArticleID(raw)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}
	if aid == 0 {
		resp.FailWithMessage(ctx, "参数有误")
		return
	}

	stat, err := service.GetArticleArchiveStat(ctx, aid)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回给前端
	resp.OkWithData(ctx, gin.H{"stat": stat})
}

// 视频分享计数
func ShareVideo(ctx *gin.Context) {
	var req dto.ShareVideoReq
	if err := ctx.Bind(&req); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	if err := service.ShareVideo(ctx, req.Vid); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.Ok(ctx)
}

// 文章分享计数
func ShareArticle(ctx *gin.Context) {
	var req dto.ShareArticleReq
	if err := ctx.Bind(&req); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	if err := service.ShareArticle(ctx, req.Aid); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	resp.Ok(ctx)
}
