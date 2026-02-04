package middleware

import (
	"interastral-peace.com/alnitak/internal/resp"

	"github.com/gin-gonic/gin"
)

func RequirePGC() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uid, exists := ctx.Get("userId")
		if !exists || uid == 0 {
			resp.Result(ctx, 3000, nil, "未登录")
			ctx.Abort()
			return
		}

		roleCode, _ := ctx.Get("roleCode")
		if roleCode != "001" && roleCode != "002" && roleCode != "003" {
			resp.FailWithMessage(ctx, "无PGC操作权限")
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}
