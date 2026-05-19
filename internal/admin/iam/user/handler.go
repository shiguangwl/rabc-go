package user

import (
	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
	"rabc-go/pkg/jwt"
)

// Handler 承载 user 子域 CRUD 接口。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetAdminUser godoc
// @Summary 获取管理用户信息
// @Tags 用户模块
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} apiv1.GetAdminUserResponse
// @Router /v1/admin/user [get]
func (h *Handler) GetAdminUser(ctx *gin.Context) {
	uid, ok := jwt.UserIDFromCtx(ctx)
	if !ok {
		apiv1.WriteResponse(ctx, apiv1.ErrUnauthorized, nil)
		return
	}
	data, err := h.svc.GetAdminUser(ctx, uid)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// GetAdminUsers godoc
// @Summary 获取管理员用户列表
// @Description 获取管理员用户列表
// @Tags 用户模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int true "页码"
// @Param pageSize query int true "每页数量"
// @Param username query string false "用户名"
// @Param nickname query string false "昵称"
// @Param phone query string false "手机号"
// @Param email query string false "邮箱"
// @Success 200 {object} apiv1.GetAdminUsersResponse
// @Router /v1/admin/users [get]
func (h *Handler) GetAdminUsers(ctx *gin.Context) {
	var req apiv1.GetAdminUsersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	data, err := h.svc.GetAdminUsers(ctx, &req)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// AdminUserUpdate godoc
// @Summary 更新管理员用户
// @Description 更新管理员用户信息
// @Tags 用户模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.AdminUserUpdateRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/user [put]
func (h *Handler) AdminUserUpdate(ctx *gin.Context) {
	var req apiv1.AdminUserUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.AdminUserUpdate(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// AdminUserCreate godoc
// @Summary 创建管理员用户
// @Description 创建新的管理员用户
// @Tags 用户模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.AdminUserCreateRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/user [post]
func (h *Handler) AdminUserCreate(ctx *gin.Context) {
	var req apiv1.AdminUserCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.AdminUserCreate(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// AdminUserDelete godoc
// @Summary 删除管理员用户
// @Description 删除指定管理员用户
// @Tags 用户模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param id query uint true "用户ID"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/user [delete]
func (h *Handler) AdminUserDelete(ctx *gin.Context) {
	var req apiv1.AdminUserDeleteRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.AdminUserDelete(ctx, req.ID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}
