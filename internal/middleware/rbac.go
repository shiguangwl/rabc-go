package middleware

import (
	"strconv"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	v1 "nunu-layout-admin/api/v1"
	"nunu-layout-admin/internal/model"
	"nunu-layout-admin/pkg/jwt"
)

func AuthMiddleware(e *casbin.SyncedEnforcer) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		v, exists := ctx.Get("claims")
		if !exists {
			v1.WriteResponse(ctx, v1.ErrUnauthorized, nil)
			ctx.Abort()
			return
		}
		userInfo, ok := v.(*jwt.MyCustomClaims)
		if !ok {
			v1.WriteResponse(ctx, v1.ErrUnauthorized, nil)
			ctx.Abort()
			return
		}
		uid := userInfo.UserID
		if strconv.FormatUint(uint64(uid), 10) == model.AdminUserID {
			// 防呆设计，超管跳过 API 权限检查
			ctx.Next()
			return
		}

		sub := strconv.FormatUint(uint64(uid), 10)
		obj := model.ApiResourcePrefix + ctx.Request.URL.Path
		act := ctx.Request.Method

		// Enforce 出错代表 Casbin 内部异常（数据库不可达等），不是"无权限"。
		// 区分语义：err != nil → 500（鉴权器故障，需告警）；!allowed → 403（无权限）。
		allowed, err := e.Enforce(sub, obj, act)
		if err != nil {
			v1.WriteResponse(ctx, v1.ErrInternalServerError.WithCause(err), nil)
			ctx.Abort()
			return
		}
		if !allowed {
			v1.WriteResponse(ctx, v1.ErrForbidden, nil)
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}
