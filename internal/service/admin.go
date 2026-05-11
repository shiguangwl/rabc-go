package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	v1 "rabc-go/api/v1"
	"rabc-go/internal/model"
	"rabc-go/internal/repository"
)

type AdminService interface {
	Login(ctx context.Context, req *v1.LoginRequest) (string, error)
	GetAdminUsers(ctx context.Context, req *v1.GetAdminUsersRequest) (*v1.GetAdminUsersResponseData, error)
	GetAdminUser(ctx context.Context, uid uint) (*v1.GetAdminUserResponseData, error)
	AdminUserUpdate(ctx context.Context, req *v1.AdminUserUpdateRequest) error
	AdminUserCreate(ctx context.Context, req *v1.AdminUserCreateRequest) error
	AdminUserDelete(ctx context.Context, id uint) error

	GetUserPermissions(ctx context.Context, uid uint) (*v1.GetUserPermissionsData, error)
	GetRolePermissions(ctx context.Context, role string) (*v1.GetRolePermissionsData, error)
	UpdateRolePermission(ctx context.Context, req *v1.UpdateRolePermissionRequest) error

	GetAdminMenus(ctx context.Context) (*v1.GetMenuResponseData, error)
	GetMenus(ctx context.Context, uid uint) (*v1.GetMenuResponseData, error)
	MenuUpdate(ctx context.Context, req *v1.MenuUpdateRequest) error
	MenuCreate(ctx context.Context, req *v1.MenuCreateRequest) error
	MenuDelete(ctx context.Context, id uint) error

	GetRoles(ctx context.Context, req *v1.GetRoleListRequest) (*v1.GetRolesResponseData, error)
	RoleUpdate(ctx context.Context, req *v1.RoleUpdateRequest) error
	RoleCreate(ctx context.Context, req *v1.RoleCreateRequest) error
	RoleDelete(ctx context.Context, id uint) error

	GetApis(ctx context.Context, req *v1.GetApisRequest) (*v1.GetApisResponseData, error)
	ApiUpdate(ctx context.Context, req *v1.ApiUpdateRequest) error
	ApiCreate(ctx context.Context, req *v1.ApiCreateRequest) error
	ApiDelete(ctx context.Context, id uint) error
}

func NewAdminService(
	service *Service,
	adminRepository repository.AdminRepository,
) AdminService {
	return &adminService{
		Service:         service,
		adminRepository: adminRepository,
	}
}

type adminService struct {
	*Service
	adminRepository repository.AdminRepository
}

const dummyPasswordHash = "$2a$10$C6UzMDM.H6dfI/f/IKcEeO6DGw4ZSLiZUj2Ip7yUpfI2KI2Zg7W6e"

func (s *adminService) GetAdminUser(ctx context.Context, uid uint) (*v1.GetAdminUserResponseData, error) {
	user, err := s.adminRepository.GetAdminUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	// 详情页查不到角色不阻塞主信息返回，但记录 warn 让 SRE 能感知到 Casbin 异常；
	// 静默 _ 忽略会让前端永远不知道角色查询挂掉。
	roles, err := s.adminRepository.GetUserRoles(ctx, uid)
	if err != nil {
		s.logger.WithContext(ctx).Warn("GetUserRoles failed in GetAdminUser", zap.Uint("uid", uid), zap.Error(err))
		roles = []string{}
	}

	return &v1.GetAdminUserResponseData{
		Email:     user.Email,
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Phone:     user.Phone,
		Roles:     roles,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: user.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *adminService) Login(ctx context.Context, req *v1.LoginRequest) (string, error) {
	user, err := s.adminRepository.GetAdminUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 用户名不存在时仍对固定 dummy hash 跑一次 bcrypt 比较，
			// 统一耗时，消除用户名是否存在的时序侧信道。
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
			return "", v1.ErrUnauthorized
		}
		return "", v1.ErrInternalServerError.WithCause(err)
	}

	// 密码不匹配语义上等同于"账号或密码错误"，对外返回 401（与 RecordNotFound 保持一致），
	// 不能裸返 bcrypt 错让 WriteResponse 兜底成 500——前端会误以为是服务异常。
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return "", v1.ErrUnauthorized
		}
		return "", v1.ErrInternalServerError.WithCause(err)
	}
	token, err := s.jwt.GenToken(user.ID, time.Now().Add(time.Hour*24*90))
	if err != nil {
		return "", v1.ErrInternalServerError.WithCause(err)
	}

	return token, nil
}

func (s *adminService) GetAdminUsers(ctx context.Context, req *v1.GetAdminUsersRequest) (*v1.GetAdminUsersResponseData, error) {
	req.Normalize()
	list, total, err := s.adminRepository.GetAdminUsers(ctx, req)
	if err != nil {
		return nil, err
	}
	data := &v1.GetAdminUsersResponseData{
		List:  make([]v1.AdminUserDataItem, 0),
		Total: total,
	}
	for _, user := range list {
		// 任一用户的角色查询失败都整体返回错误：
		// 之前的 continue 会让 List 长度小于 Total，前端分页错位、漏行难定位。
		roles, err := s.adminRepository.GetUserRoles(ctx, user.ID)
		if err != nil {
			s.logger.WithContext(ctx).Error("GetUserRoles error", zap.Uint("uid", user.ID), zap.Error(err))
			return nil, err
		}
		data.List = append(data.List, v1.AdminUserDataItem{
			Email:     user.Email,
			ID:        user.ID,
			Nickname:  user.Nickname,
			Username:  user.Username,
			Phone:     user.Phone,
			Roles:     roles,
			CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: user.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return data, nil
}

func (s *adminService) AdminUserUpdate(ctx context.Context, req *v1.AdminUserUpdateRequest) error {
	// 密码为空 = 不修改密码列。空密码不在这里读旧值回写：
	// 旧实现会在事务外做 read-modify-write，并发更新会丢密码（A 不改密码读到 H1，
	// B 改密码写入 H2，A 把 H1 写回会把 H2 静默覆盖）。空密码下不写 password 列
	// 的语义改在 repo 层按字段动态构造 map 实现。
	passwordHash := ""
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		passwordHash = string(hash)
	}
	return s.adminRepository.AdminUserUpdateAtomic(ctx, &model.AdminUser{
		Model:    gorm.Model{ID: req.ID},
		Email:    req.Email,
		Nickname: req.Nickname,
		Password: passwordHash,
		Phone:    req.Phone,
		Username: req.Username,
	}, req.Roles) // req.Roles 为 *[]string：nil 跳过角色同步，非 nil 全量覆盖
}

func (s *adminService) AdminUserCreate(ctx context.Context, req *v1.AdminUserCreateRequest) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.adminRepository.AdminUserCreateAtomic(ctx, &model.AdminUser{
		Email:    req.Email,
		Nickname: req.Nickname,
		Phone:    req.Phone,
		Username: req.Username,
		Password: string(hash),
	}, req.Roles)
}

func (s *adminService) AdminUserDelete(ctx context.Context, id uint) error {
	return s.adminRepository.AdminUserDeleteAtomic(ctx, id)
}

func (s *adminService) UpdateRolePermission(ctx context.Context, req *v1.UpdateRolePermissionRequest) error {
	permissions := map[string]struct{}{}
	for _, v := range req.List {
		if !isValidPermission(v) {
			return v1.ErrBadRequest
		}
		permissions[v] = struct{}{}
	}
	return s.adminRepository.UpdateRolePermission(ctx, req.Role, permissions)
}

// 校验权限串格式：API 权限必须携带 HTTP method，菜单权限只接受 read。
func isValidPermission(raw string) bool {
	parts := strings.Split(raw, model.PermSep)
	if len(parts) != 2 {
		return false
	}
	resource, action := parts[0], parts[1]
	if resource == "" || action == "" {
		return false
	}
	switch {
	case strings.HasPrefix(resource, model.ApiResourcePrefix):
		switch action {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead:
			return true
		default:
			return false
		}
	case strings.HasPrefix(resource, model.MenuResourcePrefix):
		return action == "read"
	default:
		return false
	}
}

func (s *adminService) GetApis(ctx context.Context, req *v1.GetApisRequest) (*v1.GetApisResponseData, error) {
	req.Normalize()
	list, total, err := s.adminRepository.GetApis(ctx, req)
	if err != nil {
		return nil, err
	}
	groups, err := s.adminRepository.GetApiGroups(ctx)
	if err != nil {
		return nil, err
	}
	data := &v1.GetApisResponseData{
		List:   make([]v1.ApiDataItem, 0),
		Total:  total,
		Groups: groups,
	}
	for _, api := range list {
		data.List = append(data.List, v1.ApiDataItem{
			CreatedAt: api.CreatedAt.Format("2006-01-02 15:04:05"),
			Group:     api.Group,
			ID:        api.ID,
			Method:    api.Method,
			Name:      api.Name,
			Path:      api.Path,
			UpdatedAt: api.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return data, nil
}

func (s *adminService) ApiUpdate(ctx context.Context, req *v1.ApiUpdateRequest) error {
	// path/method 变更时 repo 在同一事务内读取旧值并清旧 Casbin 策略；
	// 避免 service 先读旧值再更新导致并发修改时清错资源 key。
	return s.adminRepository.ApiUpdateAtomic(ctx, &model.Api{
		Group:  req.Group,
		Method: req.Method,
		Name:   req.Name,
		Path:   req.Path,
		Model:  gorm.Model{ID: req.ID},
	})
}

// ApiCreate 不需要事务：新登记的 API 资源默认无任何 Casbin 策略绑定，
// 需要管理员通过"角色权限管理"页面显式分配。直接走 repo 单写即可。
func (s *adminService) ApiCreate(ctx context.Context, req *v1.ApiCreateRequest) error {
	return s.adminRepository.ApiCreate(ctx, &model.Api{
		Group:  req.Group,
		Method: req.Method,
		Name:   req.Name,
		Path:   req.Path,
	})
}

func (s *adminService) ApiDelete(ctx context.Context, id uint) error {
	return s.adminRepository.ApiDeleteAtomic(ctx, id)
}

func (s *adminService) GetUserPermissions(ctx context.Context, uid uint) (*v1.GetUserPermissionsData, error) {
	data := &v1.GetUserPermissionsData{
		List: []string{},
	}
	list, err := s.adminRepository.GetUserPermissions(ctx, uid)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if len(v) == 3 {
			data.List = append(data.List, strings.Join([]string{v[1], v[2]}, model.PermSep))
		}
	}
	return data, nil
}
func (s *adminService) GetRolePermissions(ctx context.Context, role string) (*v1.GetRolePermissionsData, error) {
	data := &v1.GetRolePermissionsData{
		List: []string{},
	}
	list, err := s.adminRepository.GetRolePermissions(ctx, role)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if len(v) == 3 {
			data.List = append(data.List, strings.Join([]string{v[1], v[2]}, model.PermSep))
		}
	}
	return data, nil
}

func (s *adminService) MenuUpdate(ctx context.Context, req *v1.MenuUpdateRequest) error {
	return s.adminRepository.MenuUpdateAtomic(ctx, &model.Menu{
		Component:  req.Component,
		Icon:       req.Icon,
		KeepAlive:  req.KeepAlive,
		HideInMenu: req.HideInMenu,
		Locale:     req.Locale,
		Weight:     req.Weight,
		Name:       req.Name,
		ParentID:   req.ParentID,
		Path:       req.Path,
		Redirect:   req.Redirect,
		Title:      req.Title,
		URL:        req.URL,
		Model: gorm.Model{
			ID: req.ID,
		},
	})
}

func (s *adminService) MenuCreate(ctx context.Context, req *v1.MenuCreateRequest) error {
	return s.adminRepository.MenuCreate(ctx, &model.Menu{
		Component:  req.Component,
		Icon:       req.Icon,
		KeepAlive:  req.KeepAlive,
		HideInMenu: req.HideInMenu,
		Locale:     req.Locale,
		Weight:     req.Weight,
		Name:       req.Name,
		ParentID:   req.ParentID,
		Path:       req.Path,
		Redirect:   req.Redirect,
		Title:      req.Title,
		URL:        req.URL,
	})
}

func (s *adminService) MenuDelete(ctx context.Context, id uint) error {
	return s.adminRepository.MenuDeleteAtomic(ctx, id)
}

func (s *adminService) GetMenus(ctx context.Context, uid uint) (*v1.GetMenuResponseData, error) {
	menuList, err := s.adminRepository.GetMenuList(ctx)
	if err != nil {
		s.logger.WithContext(ctx).Error("GetMenuList error", zap.Error(err))
		return nil, err
	}
	data := &v1.GetMenuResponseData{
		List: make([]v1.MenuDataItem, 0),
	}
	isAdmin := strconv.FormatUint(uint64(uid), 10) == model.AdminUserID
	if isAdmin {
		for _, menu := range menuList {
			data.List = append(data.List, v1.MenuDataItem{
				ID:         menu.ID,
				Name:       menu.Name,
				Title:      menu.Title,
				Path:       menu.Path,
				Component:  menu.Component,
				Redirect:   menu.Redirect,
				KeepAlive:  menu.KeepAlive,
				HideInMenu: menu.HideInMenu,
				Locale:     menu.Locale,
				Weight:     menu.Weight,
				Icon:       menu.Icon,
				ParentID:   menu.ParentID,
				UpdatedAt:  menu.UpdatedAt.Format("2006-01-02 15:04:05"),
				URL:        menu.URL,
			})
		}
		return data, nil
	}

	// 获取权限的菜单
	permissions, err := s.adminRepository.GetUserPermissions(ctx, uid)
	if err != nil {
		return nil, err
	}
	menuPermMap := map[string]struct{}{}
	for _, permission := range permissions {
		// permission 格式预期为 [sub, obj, act]；任何长度<2 的脏数据直接跳过，避免越界。
		if len(permission) < 2 {
			continue
		}
		obj := permission[1]
		if !strings.HasPrefix(obj, model.MenuResourcePrefix) {
			continue
		}
		if len(permission) != 3 {
			continue
		}
		menuPermMap[strings.TrimPrefix(obj, model.MenuResourcePrefix)] = struct{}{}
	}

	for _, menu := range menuList {
		if _, ok := menuPermMap[menu.Path]; ok {
			data.List = append(data.List, v1.MenuDataItem{
				ID:         menu.ID,
				Name:       menu.Name,
				Title:      menu.Title,
				Path:       menu.Path,
				Component:  menu.Component,
				Redirect:   menu.Redirect,
				KeepAlive:  menu.KeepAlive,
				HideInMenu: menu.HideInMenu,
				Locale:     menu.Locale,
				Weight:     menu.Weight,
				Icon:       menu.Icon,
				ParentID:   menu.ParentID,
				UpdatedAt:  menu.UpdatedAt.Format("2006-01-02 15:04:05"),
				URL:        menu.URL,
			})
		}
	}
	return data, nil
}
func (s *adminService) GetAdminMenus(ctx context.Context) (*v1.GetMenuResponseData, error) {
	menuList, err := s.adminRepository.GetMenuList(ctx)
	if err != nil {
		s.logger.WithContext(ctx).Error("GetMenuList error", zap.Error(err))
		return nil, err
	}
	data := &v1.GetMenuResponseData{
		List: make([]v1.MenuDataItem, 0),
	}
	for _, menu := range menuList {
		data.List = append(data.List, v1.MenuDataItem{
			ID:         menu.ID,
			Name:       menu.Name,
			Title:      menu.Title,
			Path:       menu.Path,
			Component:  menu.Component,
			Redirect:   menu.Redirect,
			KeepAlive:  menu.KeepAlive,
			HideInMenu: menu.HideInMenu,
			Locale:     menu.Locale,
			Weight:     menu.Weight,
			Icon:       menu.Icon,
			ParentID:   menu.ParentID,
			UpdatedAt:  menu.UpdatedAt.Format("2006-01-02 15:04:05"),
			URL:        menu.URL,
		})
	}
	return data, nil
}

// RoleUpdate 仅更新角色显示名 Name；Sid 是 Casbin 策略的关联键，
// 一旦修改会让所有 g/p 行指向不存在的 role，导致权限静默失效，因此禁止变更。
// repo 层用 UpdateColumn("name", ...) 显式只写一列，即使 req.Sid 非空也不会落库。
func (s *adminService) RoleUpdate(ctx context.Context, req *v1.RoleUpdateRequest) error {
	return s.adminRepository.RoleUpdate(ctx, &model.Role{
		Name: req.Name,
		Model: gorm.Model{
			ID: req.ID,
		},
	})
}

func (s *adminService) RoleCreate(ctx context.Context, req *v1.RoleCreateRequest) error {
	// 禁止 sid 以 RoleSubjectPrefix 开头：Casbin 内部 role subject 形如 "role:<sid>"，
	// 若外部传入 sid="role:foo" 会与正常 sid="foo" 的 RoleSubject 结果撞名，
	// 让命名空间隔离失效。model.RoleSubject 出于幂等保留了短路逻辑，
	// 真正堵漏点在入口校验。
	if strings.HasPrefix(req.Sid, model.RoleSubjectPrefix) {
		return v1.ErrBadRequest
	}
	// 唯一约束冲突由 repo 反查精确翻译为 ErrRoleSidExists / ErrRoleNameExists；
	// service 层只透传业务错误，不再依赖 affected 行数语义。
	return s.adminRepository.RoleCreateIfAbsent(ctx, &model.Role{Name: req.Name, Sid: req.Sid})
}

func (s *adminService) RoleDelete(ctx context.Context, id uint) error {
	old, err := s.adminRepository.GetRole(ctx, id)
	if err != nil {
		return err
	}
	return s.adminRepository.RoleDeleteAtomic(ctx, id, old.Sid)
}

func (s *adminService) GetRoles(ctx context.Context, req *v1.GetRoleListRequest) (*v1.GetRolesResponseData, error) {
	req.Normalize()
	list, total, err := s.adminRepository.GetRoles(ctx, req)
	if err != nil {
		return nil, err
	}
	data := &v1.GetRolesResponseData{
		List:  make([]v1.RoleDataItem, 0),
		Total: total,
	}
	for _, role := range list {
		data.List = append(data.List, v1.RoleDataItem{
			ID:        role.ID,
			Name:      role.Name,
			Sid:       role.Sid,
			UpdatedAt: role.UpdatedAt.Format("2006-01-02 15:04:05"),
			CreatedAt: role.CreatedAt.Format("2006-01-02 15:04:05"),
		})

	}
	return data, nil
}
