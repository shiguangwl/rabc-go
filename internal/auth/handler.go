package auth

import (
	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
)

// Handler 承载认证相关接口：Login / Refresh / Logout 及 session 管理（GetUserSessions / RevokeUserSessions / KickSession）。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Login godoc
// @Summary 账号登录
// @Tags 用户模块
// @Accept json
// @Produce json
// @Param request body apiv1.LoginRequest true "params"
// @Success 200 {object} apiv1.LoginResponse
// @Router /v1/login [post]
func (h *Handler) Login(ctx *gin.Context) {
	var req apiv1.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	result, err := h.svc.Login(ctx, &req)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, apiv1.LoginResponseData{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
	})
}

// Refresh godoc
// @Summary 刷新访问令牌
// @Description 用 refresh token 换取新 access + 新 refresh（轮换语义）；旧 refresh 立即失效。
// @Tags 用户模块
// @Accept json
// @Produce json
// @Param request body apiv1.RefreshRequest true "params"
// @Success 200 {object} apiv1.RefreshResponse
// @Router /v1/auth/refresh [post]
func (h *Handler) Refresh(ctx *gin.Context) {
	var req apiv1.RefreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	result, err := h.svc.Refresh(ctx, &req)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, apiv1.RefreshResponseData{
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
// @Param request body apiv1.LogoutRequest true "params"
// @Success 200 {object} apiv1.Response
// @Router /v1/auth/logout [post]
func (h *Handler) Logout(ctx *gin.Context) {
	var req apiv1.LogoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.Logout(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// GetUserSessions godoc
// @Summary 获取用户活跃会话
// @Tags 用户模块
// @Produce json
// @Security Bearer
// @Param id query uint true "用户ID"
// @Success 200 {object} apiv1.GetUserSessionsResponse
// @Router /v1/admin/user/sessions [get]
func (h *Handler) GetUserSessions(ctx *gin.Context) {
	var req apiv1.GetUserSessionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	sessions, err := h.svc.ListUserSessions(ctx, req.ID)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	out := make([]apiv1.UserSessionItem, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, apiv1.UserSessionItem{SID: s.SID, Exp: s.Exp})
	}
	apiv1.HandleSuccess(ctx, apiv1.GetUserSessionsResponseData{List: out})
}

// RevokeUserSessions godoc
// @Summary 踢出用户全部会话
// @Tags 用户模块
// @Produce json
// @Security Bearer
// @Param id query uint true "用户ID"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/user/sessions [delete]
func (h *Handler) RevokeUserSessions(ctx *gin.Context) {
	var req apiv1.RevokeUserSessionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if _, err := h.svc.RevokeAllUserSessions(ctx, req.ID, "admin_kick_all"); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// KickUserSession godoc
// @Summary 踢下线单个会话
// @Tags 用户模块
// @Produce json
// @Security Bearer
// @Param id query uint true "用户ID"
// @Param sessionID query string true "会话ID"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/user/session [delete]
func (h *Handler) KickUserSession(ctx *gin.Context) {
	var req apiv1.KickUserSessionRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.KickSession(ctx, req.ID, req.SessionID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}
