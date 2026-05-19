package role

import (
	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
)

// Handler 承载 role 子域 CRUD 接口。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetRoles godoc
// @Summary 获取角色列表
// @Description 获取角色列表
// @Tags 角色模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int true "页码"
// @Param pageSize query int true "每页数量"
// @Param sid query string false "角色ID"
// @Param name query string false "角色名称"
// @Success 200 {object} apiv1.GetRolesResponse
// @Router /v1/admin/roles [get]
func (h *Handler) GetRoles(ctx *gin.Context) {
	var req apiv1.GetRoleListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	data, err := h.svc.GetRoles(ctx, &req)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// RoleCreate godoc
// @Summary 创建角色
// @Description 创建新的角色
// @Tags 角色模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.RoleCreateRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/role [post]
func (h *Handler) RoleCreate(ctx *gin.Context) {
	var req apiv1.RoleCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.RoleCreate(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// RoleUpdate godoc
// @Summary 更新角色
// @Description 更新角色信息
// @Tags 角色模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.RoleUpdateRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/role [put]
func (h *Handler) RoleUpdate(ctx *gin.Context) {
	var req apiv1.RoleUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.RoleUpdate(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// RoleDelete godoc
// @Summary 删除角色
// @Description 删除指定角色
// @Tags 角色模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param id query uint true "角色ID"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/role [delete]
func (h *Handler) RoleDelete(ctx *gin.Context) {
	var req apiv1.RoleDeleteRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.RoleDelete(ctx, req.ID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}
