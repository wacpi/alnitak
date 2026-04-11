package api

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/resp"
	"interastral-peace.com/alnitak/internal/service"
	"interastral-peace.com/alnitak/utils"
)

// GetReadStatus 获取公告/点赞/回复/@ 已读进度（供客户端清数据后恢复）
func GetReadStatus(ctx *gin.Context) {
	data := service.GetReadStatus(ctx)
	resp.OkWithData(ctx, data)
}

// SaveReadStatus 上报某分类已读到的消息 ID
func SaveReadStatus(ctx *gin.Context) {
	var req dto.ReadStatusSaveReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.FailWithMessage(ctx, "请求参数有误")
		return
	}
	if err := service.SaveReadStatus(ctx, req.Category, req.ReadUpToId); err != nil {
		utils.ErrorLog("保存消息已读进度失败", "readStatus", err.Error())
		resp.FailWithMessage(ctx, "保存失败")
		return
	}
	resp.Ok(ctx)
}
