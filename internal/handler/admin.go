package handler

import (
	"github.com/gin-gonic/gin"

	"rabc-go/api/apiv1"
	"rabc-go/internal/service"
)

type AdminHandler struct {
	*Handler
	adminService service.AdminService
	// authService 处理登录（双 Token 颁发：access + refresh + expiresIn）。
	// AdminHandler 同时持有 adminService 和 authService：admin 业务路径仍走 adminService，
	// 仅 /v1/login 走 authService。AuthHandler 负责 /v1/auth/refresh + /v1/auth/logout。
	authService service.AuthService
}

func NewAdminHandler(
	handler *Handler,
	adminService service.AdminService,
	authService service.AuthService,
) *AdminHandler {
	return &AdminHandler{
		Handler:      handler,
		adminService: adminService,
		authService:  authService,
	}
}

// Login godoc
// @Summary 账号登录
// @Tags 用户模块
// @Accept json
// @Produce json
// @Param request body apiv1.LoginRequest true "params"
// @Success 200 {object} apiv1.LoginResponse
// @Router /v1/login [post]
func (h *AdminHandler) Login(ctx *gin.Context) {
	var req apiv1.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}

	result, err := h.authService.Login(ctx, &req)
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

// GetMenus godoc
// @Summary 获取用户菜单
// @Description 获取当前用户的菜单列表
// @Tags 菜单模块
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} apiv1.GetMenuResponse
// @Router /v1/menus [get]
func (h *AdminHandler) GetMenus(ctx *gin.Context) {
	uid, ok := UserIDFromCtx(ctx)
	if !ok {
		apiv1.WriteResponse(ctx, apiv1.ErrUnauthorized, nil)
		return
	}
	data, err := h.adminService.GetMenus(ctx, uid)
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
func (h *AdminHandler) GetAdminMenus(ctx *gin.Context) {
	data, err := h.adminService.GetAdminMenus(ctx)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
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
func (h *AdminHandler) GetUserPermissions(ctx *gin.Context) {
	uid, ok := UserIDFromCtx(ctx)
	if !ok {
		apiv1.WriteResponse(ctx, apiv1.ErrUnauthorized, nil)
		return
	}
	data, err := h.adminService.GetUserPermissions(ctx, uid)
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
func (h *AdminHandler) GetRolePermissions(ctx *gin.Context) {
	var req apiv1.GetRolePermissionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	data, err := h.adminService.GetRolePermissions(ctx, req.Role)
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
func (h *AdminHandler) UpdateRolePermission(ctx *gin.Context) {
	var req apiv1.UpdateRolePermissionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.UpdateRolePermission(ctx, &req); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
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
func (h *AdminHandler) MenuUpdate(ctx *gin.Context) {
	var req apiv1.MenuUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.MenuUpdate(ctx, &req); err != nil {
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
func (h *AdminHandler) MenuCreate(ctx *gin.Context) {
	var req apiv1.MenuCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.MenuCreate(ctx, &req); err != nil {
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
func (h *AdminHandler) MenuDelete(ctx *gin.Context) {
	var req apiv1.MenuDeleteRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.MenuDelete(ctx, req.ID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
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
func (h *AdminHandler) GetRoles(ctx *gin.Context) {
	var req apiv1.GetRoleListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	data, err := h.adminService.GetRoles(ctx, &req)
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
func (h *AdminHandler) RoleCreate(ctx *gin.Context) {
	var req apiv1.RoleCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.RoleCreate(ctx, &req); err != nil {
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
func (h *AdminHandler) RoleUpdate(ctx *gin.Context) {
	var req apiv1.RoleUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.RoleUpdate(ctx, &req); err != nil {
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
func (h *AdminHandler) RoleDelete(ctx *gin.Context) {
	var req apiv1.RoleDeleteRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.RoleDelete(ctx, req.ID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
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
func (h *AdminHandler) GetApis(ctx *gin.Context) {
	var req apiv1.GetApisRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	data, err := h.adminService.GetApis(ctx, &req)
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
func (h *AdminHandler) APICreate(ctx *gin.Context) {
	var req apiv1.APICreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.APICreate(ctx, &req); err != nil {
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
func (h *AdminHandler) APIUpdate(ctx *gin.Context) {
	var req apiv1.APIUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.APIUpdate(ctx, &req); err != nil {
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
func (h *AdminHandler) APIDelete(ctx *gin.Context) {
	var req apiv1.APIDeleteRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.APIDelete(ctx, req.ID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
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
func (h *AdminHandler) AdminUserUpdate(ctx *gin.Context) {
	var req apiv1.AdminUserUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.AdminUserUpdate(ctx, &req); err != nil {
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
func (h *AdminHandler) AdminUserCreate(ctx *gin.Context) {
	var req apiv1.AdminUserCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.AdminUserCreate(ctx, &req); err != nil {
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
func (h *AdminHandler) AdminUserDelete(ctx *gin.Context) {
	var req apiv1.AdminUserDeleteRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.adminService.AdminUserDelete(ctx, req.ID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
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
func (h *AdminHandler) GetAdminUsers(ctx *gin.Context) {
	var req apiv1.GetAdminUsersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	data, err := h.adminService.GetAdminUsers(ctx, &req)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}

// GetUserSessions godoc
// @Summary 获取用户活跃会话
// @Tags 用户模块
// @Produce json
// @Security Bearer
// @Param id query uint true "用户ID"
// @Success 200 {object} apiv1.GetUserSessionsResponse
// @Router /v1/admin/user/sessions [get]
func (h *AdminHandler) GetUserSessions(ctx *gin.Context) {
	var req apiv1.GetUserSessionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	sessions, err := h.authService.ListUserSessions(ctx, req.ID)
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
func (h *AdminHandler) RevokeUserSessions(ctx *gin.Context) {
	var req apiv1.RevokeUserSessionsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if _, err := h.authService.RevokeAllUserSessions(ctx, req.ID, "admin_kick_all"); err != nil {
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
func (h *AdminHandler) KickUserSession(ctx *gin.Context) {
	var req apiv1.KickUserSessionRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
		return
	}
	if err := h.authService.KickSession(ctx, req.ID, req.SessionID); err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, nil)
}

// GetAdminUser godoc
// @Summary 获取管理用户信息
// @Tags 用户模块
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} apiv1.GetAdminUserResponse
// @Router /v1/admin/user [get]
func (h *AdminHandler) GetAdminUser(ctx *gin.Context) {
	uid, ok := UserIDFromCtx(ctx)
	if !ok {
		apiv1.WriteResponse(ctx, apiv1.ErrUnauthorized, nil)
		return
	}
	data, err := h.adminService.GetAdminUser(ctx, uid)
	if err != nil {
		apiv1.WriteResponse(ctx, err, nil)
		return
	}
	apiv1.HandleSuccess(ctx, data)
}
