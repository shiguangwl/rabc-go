package handler

import (
	"github.com/gin-gonic/gin"
	"nunu-layout-admin/pkg/jwt"
	"nunu-layout-admin/pkg/log"
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
func GetUserIdFromCtx(ctx *gin.Context) uint {
	v, exists := ctx.Get("claims")
	if !exists {
		return 0
	}
	return v.(*jwt.MyCustomClaims).UserId
}
