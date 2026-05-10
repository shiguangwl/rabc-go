package middleware

import (
	"crypto/md5"
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	v1 "nunu-layout-admin/api/v1"
	"nunu-layout-admin/pkg/log"
)

func SignMiddleware(logger *log.Logger, conf *viper.Viper) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requiredHeaders := []string{"Timestamp", "Nonce", "Sign", "App-Version"}

		for _, header := range requiredHeaders {
			value, ok := ctx.Request.Header[header]
			if !ok || len(value) == 0 {
				v1.WriteResponse(ctx, v1.ErrBadRequest, nil)
				ctx.Abort()
				return
			}
		}

		appSecret := conf.GetString("security.api_sign.app_security")
		// 固定 4 字段按 key 字典序拼接：AppKey < AppVersion < Nonce < Timestamp
		var sb strings.Builder
		sb.WriteString("AppKey")
		sb.WriteString(conf.GetString("security.api_sign.app_key"))
		sb.WriteString("AppVersion")
		sb.WriteString(ctx.Request.Header.Get("App-Version"))
		sb.WriteString("Nonce")
		sb.WriteString(ctx.Request.Header.Get("Nonce"))
		sb.WriteString("Timestamp")
		sb.WriteString(ctx.Request.Header.Get("Timestamp"))
		sb.WriteString(appSecret)

		expected := strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte(sb.String()))))
		actual := ctx.Request.Header.Get("Sign")

		if subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) != 1 {
			v1.WriteResponse(ctx, v1.ErrBadRequest, nil)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
