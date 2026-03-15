package api

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/resp"
	"interastral-peace.com/alnitak/internal/service"
)

func LikeArticle(ctx *gin.Context) {
	// 获取参数
	var likeReq dto.LikeArticleReq
	if err := ctx.Bind(&likeReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	if err := service.LikeArticle(ctx, likeReq); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回
	resp.Ok(ctx)
}

func CancelLikeArticle(ctx *gin.Context) {
	var likeReq dto.LikeArticleReq
	if err := ctx.Bind(&likeReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	if err := service.CancelLikeArticle(ctx, likeReq); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回
	resp.Ok(ctx)
}

func HasLikeArticle(ctx *gin.Context) {
	raw := ctx.Query("aid")
	articleId, err := service.ParseArticleID(raw)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}
	if articleId == 0 {
		resp.FailWithMessage(ctx, "参数有误")
		return
	}
	like, err := service.HasLikeArticle(ctx, articleId)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回
	resp.OkWithData(ctx, gin.H{"like": like})
}
