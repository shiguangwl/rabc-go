package handler

import (
	v1 "rabc-go/api/v1"
	"rabc-go/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	*Handler
	userService service.UserService
}

func NewUserHandler(
	handler *Handler,
	userService service.UserService,
) *UserHandler {
	return &UserHandler{
		Handler:     handler,
		userService: userService,
	}
}

func (h *UserHandler) GetUsers(ctx *gin.Context) {
	v1.WriteResponse(ctx, v1.ErrNotImplemented, nil)
}
