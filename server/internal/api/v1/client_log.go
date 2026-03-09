package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/resp"
)

// ClientLog 接收客户端上报日志（需登录鉴权，避免被滥用）
func ClientLog(ctx *gin.Context) {
	var req dto.ClientLogReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.FailWithMessage(ctx, "参数错误")
		return
	}
	msg := "client_log " + req.Message
	fields := []zap.Field{}
	if len(req.Context) > 0 {
		fields = append(fields, zap.Any("ctx", req.Context))
	}
	if req.Error != "" {
		fields = append(fields, zap.String("err", req.Error))
	}
	if req.StackTrace != "" {
		fields = append(fields, zap.String("stack", req.StackTrace))
	}
	switch req.Level {
	case "warn":
		zap.L().Warn(msg, fields...)
	case "error":
		zap.L().Error(msg, fields...)
	default:
		zap.L().Info(msg, fields...)
	}
	resp.Ok(ctx)
}
