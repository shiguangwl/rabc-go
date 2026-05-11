package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	v1 "rabc-go/api/v1"
	"rabc-go/pkg/jwt"
	"rabc-go/pkg/log"
)

func StrictAuth(j *jwt.JWT, logger *log.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenString := ctx.Request.Header.Get("Authorization")
		if tokenString == "" {
			logger.WithContext(ctx).Warn("No token", zap.Any("data", map[string]interface{}{
				"url":    ctx.Request.URL,
				"params": ctx.Params,
			}))
			v1.WriteResponse(ctx, v1.ErrUnauthorized, nil)
			ctx.Abort()
			return
		}

		claims, err := j.ParseToken(tokenString)
		if err != nil {
			logger.WithContext(ctx).Warn("token parse failed", zap.Any("data", map[string]interface{}{
				"url":    ctx.Request.URL,
				"params": ctx.Params,
			}), zap.Error(err))
			v1.WriteResponse(ctx, v1.ErrUnauthorized, nil)
			ctx.Abort()
			return
		}

		ctx.Set("claims", claims)
		injectClaimsToLogger(ctx, logger)
		ctx.Next()
	}
}

func NoStrictAuth(j *jwt.JWT, logger *log.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenString := ctx.Request.Header.Get("Authorization")
		if tokenString == "" {
			tokenString, _ = ctx.Cookie("accessToken")
		}
		if tokenString == "" {
			tokenString = ctx.Query("accessToken")
		}
		if tokenString == "" {
			ctx.Next()
			return
		}

		claims, err := j.ParseToken(tokenString)
		if err != nil {
			logger.WithContext(ctx).Warn("NoStrictAuth: token parse failed, continuing without auth", zap.Error(err))
			ctx.Next()
			return
		}

		ctx.Set("claims", claims)
		injectClaimsToLogger(ctx, logger)
		ctx.Next()
	}
}

// injectClaimsToLogger 把 JWT claims 中的 UserID 注入到 logger 上下文，
// 后续日志会自动带上 UserID 字段，便于排查。
//
// 使用 ctx.Get（而非 MustGet）避免缺失 claims 时 panic——
// 调用方必须保证仅在 ctx.Set("claims", ...) 之后调用，但即便上游遗漏，
// 这里也不会把缺失上下文升级成 500。
func injectClaimsToLogger(ctx *gin.Context, logger *log.Logger) {
	v, exists := ctx.Get("claims")
	if !exists {
		return
	}
	if userInfo, ok := v.(*jwt.MyCustomClaims); ok {
		logger.WithValue(ctx, zap.Any("UserID", userInfo.UserID))
	}
}
