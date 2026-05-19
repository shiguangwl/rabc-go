package permission

import (
	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
	"rabc-go/pkg/jwt"
)

// Handler 承载权限查询与更新接口。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetUserPermissions godoc
// @Summary 获取用户权限
// @Description 获取当前用户的权限列表
// @Tags 权限模块
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} apiv1.GetUserPermissionsData
// @Router /v1/admin/user/permissions [get]
func (h *Handler) GetUserPermissions(ctx *gin.Context) {
	uid, ok := jwt.UserIDFromCtx(ctx)
	if !ok {
		apiv1.WriteResponse(ctx, apiv1.ErrUnauthorized, nil)
		return
	}
	data, err := h.svc.GetUserPermissions(ctx, uid)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// GetRolePermissions godoc
// @Summary 获取角色权限
// @Description 获取指定角色的权限列表
// @Tags 权限模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param role query string true "角色名称"
// @Success 200 {object} apiv1.GetRolePermissionsData
// @Router /v1/admin/role/permissions [get]
func (h *Handler) GetRolePermissions(ctx *gin.Context) {
	var req apiv1.GetRolePermissionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	data, err := h.svc.GetRolePermissions(ctx, req.Role)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// UpdateRolePermission godoc
// @Summary 更新角色权限
// @Description 更新指定角色的权限列表
// @Tags 权限模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.UpdateRolePermissionRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/role/permission [put]
func (h *Handler) UpdateRolePermission(ctx *gin.Context) {
	var req apiv1.UpdateRolePermissionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.UpdateRolePermission(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}
