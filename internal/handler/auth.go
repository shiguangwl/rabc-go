// internal/handler/auth.go 处理 Auth 子系统的 HTTP 入口：
//
//	POST /v1/auth/refresh  → 刷新 access + refresh
//	POST /v1/auth/logout   → 主动登出（删除 session，不连坐）
//
// 设计 Why：与 admin Login 拆开放在独立 Handler 是因为
//  1. /v1/login 是历史路径，前端已在用，不动；
//  2. Refresh / Logout 是新协议，集中到 /v1/auth/* 命名空间便于运维识别；
//  3. Handler 仅做参数绑定与错误透传，业务全由 AuthService 处理。
package handler

import (
	"github.com/gin-gonic/gin"

	v1 "rabc-go/api/v1"
	"rabc-go/internal/service"
)

type AuthHandler struct {
	*Handler
	authService service.AuthService
}

func NewAuthHandler(handler *Handler, authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		Handler:     handler,
		authService: authService,
	}
}

// Refresh godoc
// @Summary 刷新访问令牌
// @Description 用 refresh token 换取新 access + 新 refresh（轮换语义）；旧 refresh 立即失效。
// @Tags 用户模块
// @Accept json
// @Produce json
// @Param request body v1.RefreshRequest true "params"
// @Success 200 {object} v1.RefreshResponse
// @Router /v1/auth/refresh [post]
func (h *AuthHandler) Refresh(ctx *gin.Context) {
	var req v1.RefreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.WriteResponse(ctx, v1.ErrBadRequest, nil)
		return
	}
	result, err := h.authService.Refresh(ctx, &req)
	if err != nil {
		v1.WriteResponse(ctx, err, nil)
		return
	}
	v1.HandleSuccess(ctx, v1.RefreshResponseData{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
	})
}

// Logout godoc
// @Summary 登出
// @Description 删除当前 refresh token 对应的 session；不连坐其他 session。
// @Tags 用户模块
// @Accept json
// @Produce json
// @Param request body v1.LogoutRequest true "params"
// @Success 200 {object} v1.Response
// @Router /v1/auth/logout [post]
func (h *AuthHandler) Logout(ctx *gin.Context) {
	var req v1.LogoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.WriteResponse(ctx, v1.ErrBadRequest, nil)
		return
	}
	if err := h.authService.Logout(ctx, &req); err != nil {
		v1.WriteResponse(ctx, err, nil)
		return
	}
	v1.HandleSuccess(ctx, nil)
}
