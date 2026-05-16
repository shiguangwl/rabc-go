package api

import (
	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
)

// Handler 承载 API 资源管理接口。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetApis godoc
// @Summary 获取API列表
// @Description 获取API列表
// @Tags API模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int true "页码"
// @Param pageSize query int true "每页数量"
// @Param group query string false "API分组"
// @Param name query string false "API名称"
// @Param path query string false "API路径"
// @Param method query string false "请求方法"
// @Success 200 {object} apiv1.GetApisResponse
// @Router /v1/admin/apis [get]
func (h *Handler) GetApis(ctx *gin.Context) {
	var req apiv1.GetApisRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	data, err := h.svc.GetApis(ctx, &req)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// APICreate godoc
// @Summary 创建API
// @Description 创建新的API
// @Tags API模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.APICreateRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/api [post]
func (h *Handler) APICreate(ctx *gin.Context) {
	var req apiv1.APICreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.APICreate(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// APIUpdate godoc
// @Summary 更新API
// @Description 更新API信息
// @Tags API模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.APIUpdateRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/api [put]
func (h *Handler) APIUpdate(ctx *gin.Context) {
	var req apiv1.APIUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.APIUpdate(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// APIDelete godoc
// @Summary 删除API
// @Description 删除指定API
// @Tags API模块
// @Accept json
// @Produce json
// @Security Bearer
// @Param id query uint true "API ID"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/api [delete]
func (h *Handler) APIDelete(ctx *gin.Context) {
	var req apiv1.APIDeleteRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.APIDelete(ctx, req.ID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}
