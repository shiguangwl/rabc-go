package config

import (
	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
)

// Handler 承载系统配置子域接口。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetPublicConfigs godoc
// @Summary 获取公开配置
// @Description 获取允许未登录访问的配置，供登录页等场景使用
// @Tags 系统配置
// @Accept json
// @Produce json
// @Success 200 {object} apiv1.GetPublicConfigsResponse
// @Router /v1/config/public [get]
func (h *Handler) GetPublicConfigs(ctx *gin.Context) {
	data, err := h.svc.PublicConfigs(ctx)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// GetConfigs godoc
// @Summary 获取系统配置
// @Description 按分组聚合返回全部系统配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} apiv1.GetConfigsResponse
// @Router /v1/admin/configs [get]
func (h *Handler) GetConfigs(ctx *gin.Context) {
	data, err := h.svc.GroupedConfigs(ctx)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// BatchUpdateConfig godoc
// @Summary 批量更新配置
// @Description 按分组一次性提交多个配置项的值，整批校验通过才落库
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.BatchUpdateConfigRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/configs [put]
func (h *Handler) BatchUpdateConfig(ctx *gin.Context) {
	var req apiv1.BatchUpdateConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.BatchUpdate(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// ConfigCreate godoc
// @Summary 创建配置
// @Description 新增自定义配置项
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body apiv1.ConfigCreateRequest true "参数"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/config [post]
func (h *Handler) ConfigCreate(ctx *gin.Context) {
	var req apiv1.ConfigCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.Create(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// ConfigDelete godoc
// @Summary 删除配置
// @Description 删除自定义配置项，内置配置不允许删除
// @Tags 系统配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param id query uint true "配置ID"
// @Success 200 {object} apiv1.Response
// @Router /v1/admin/config [delete]
func (h *Handler) ConfigDelete(ctx *gin.Context) {
	var req apiv1.ConfigDeleteRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.svc.Delete(ctx, req.ID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}
