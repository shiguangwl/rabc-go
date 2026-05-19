package menu

import (
	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
	"rabc-go/pkg/jwt"
)

// Handler 承载 menu 子域接口。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetMenus godoc
// @Summary 获取用户菜单
// @Description 获取当前用户的菜单列表
// @Tags 菜单模块
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} apiv1.GetMenuResponse
// @Router /v1/menus [get]
func (h *Handler) GetMenus(ctx *gin.Context) {
	uid, ok := jwt.UserIDFromCtx(ctx)
	if !ok {
		apiv1.WriteResponse(ctx, apiv1.ErrUnauthorized, nil)
		return
	}
	data, err := h.svc.GetMenus(ctx, uid)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// GetAdminMenus godoc
// @Summary 获取管理员菜单
// @Description 获取管理员菜单列表
// @Tags 菜单模块
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} apiv1.GetMenuResponse
// @Router /v1/admin/menus [get]
func (h *Handler) GetAdminMenus(ctx *gin.Context) {
	data, err := h.svc.GetAdminMenus(ctx)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// MenuUpdate godoc
// @Summary 更新菜单
// @Description 更新菜单信息
// @Tags 菜单模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.MenuUpdateRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/menu [put]
func (h *Handler) MenuUpdate(ctx *gin.Context) {
	var req apiv1.MenuUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.MenuUpdate(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// MenuCreate godoc
// @Summary 创建菜单
// @Description 创建新的菜单
// @Tags 菜单模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.MenuCreateRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/menu [post]
func (h *Handler) MenuCreate(ctx *gin.Context) {
	var req apiv1.MenuCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.MenuCreate(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// MenuDelete godoc
// @Summary 删除菜单
// @Description 删除指定菜单
// @Tags 菜单模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param id query uint true "菜单ID"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/menu [delete]
func (h *Handler) MenuDelete(ctx *gin.Context) {
	var req apiv1.MenuDeleteRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.MenuDelete(ctx, req.ID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}
