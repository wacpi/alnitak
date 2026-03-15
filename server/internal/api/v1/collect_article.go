package api

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/resp"
	"interastral-peace.com/alnitak/internal/service"
)

func CollectArticle(ctx *gin.Context) {
	// 获取参数
	var collectReq dto.CollectArticleReq
	if err := ctx.Bind(&collectReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	if err := service.CollectArticle(ctx, collectReq); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回
	resp.Ok(ctx)
}

func CancelCollectArticle(ctx *gin.Context) {
	var collectReq dto.CollectArticleReq
	if err := ctx.Bind(&collectReq); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}

	if err := service.CancelCollectArticle(ctx, collectReq); err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回
	resp.Ok(ctx)
}

func HasCollectArticle(ctx *gin.Context) {
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
	collect, err := service.HasCollectArticle(ctx, articleId)
	if err != nil {
		resp.FailWithMessage(ctx, err.Error())
		return
	}

	// 返回
	resp.OkWithData(ctx, gin.H{"collect": collect})
}
