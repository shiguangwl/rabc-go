package jwt

import "github.com/gin-gonic/gin"

// UserIDFromCtx 从 gin.Context 取 JWT 中间件注入的 claims，
// 返回 UserID + 是否有效。无 claims 或 UserID==0 视为未认证。
func UserIDFromCtx(ctx *gin.Context) (uint, bool) {
	v, exists := ctx.Get("claims")
	if !exists {
		return 0, false
	}
	claims, ok := v.(*MyCustomClaims)
	if !ok || claims.UserID == 0 {
		return 0, false
	}
	return claims.UserID, true
}
