package handler

import (
	"rabc-go/pkg/jwt"
	"rabc-go/pkg/log"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	logger *log.Logger
}

func NewHandler(
	logger *log.Logger,
) *Handler {
	return &Handler{
		logger: logger,
	}
}

func UserIDFromCtx(ctx *gin.Context) (uint, bool) {
	v, exists := ctx.Get("claims")
	if !exists {
		return 0, false
	}
	claims, ok := v.(*jwt.MyCustomClaims)
	if !ok || claims.UserID == 0 {
		return 0, false
	}
	return claims.UserID, true
}
