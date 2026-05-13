package service

import (
	"context"
	"errors"

	"rabc-go/api/apiv1"
	"rabc-go/internal/repository"
)

type AdminService interface {
	GetAdminUsers(ctx context.Context, req *apiv1.GetAdminUsersRequest) (*apiv1.GetAdminUsersResponseData, error)
	GetAdminUser(ctx context.Context, uid uint) (*apiv1.GetAdminUserResponseData, error)
	AdminUserUpdate(ctx context.Context, req *apiv1.AdminUserUpdateRequest) error
	AdminUserCreate(ctx context.Context, req *apiv1.AdminUserCreateRequest) error
	AdminUserDelete(ctx context.Context, id uint) error

	GetUserPermissions(ctx context.Context, uid uint) (*apiv1.GetUserPermissionsData, error)
	GetRolePermissions(ctx context.Context, role string) (*apiv1.GetRolePermissionsData, error)
	UpdateRolePermission(ctx context.Context, req *apiv1.UpdateRolePermissionRequest) error

	GetAdminMenus(ctx context.Context) (*apiv1.GetMenuResponseData, error)
	GetMenus(ctx context.Context, uid uint) (*apiv1.GetMenuResponseData, error)
	MenuUpdate(ctx context.Context, req *apiv1.MenuUpdateRequest) error
	MenuCreate(ctx context.Context, req *apiv1.MenuCreateRequest) error
	MenuDelete(ctx context.Context, id uint) error

	GetRoles(ctx context.Context, req *apiv1.GetRoleListRequest) (*apiv1.GetRolesResponseData, error)
	RoleUpdate(ctx context.Context, req *apiv1.RoleUpdateRequest) error
	RoleCreate(ctx context.Context, req *apiv1.RoleCreateRequest) error
	RoleDelete(ctx context.Context, id uint) error

	GetApis(ctx context.Context, req *apiv1.GetApisRequest) (*apiv1.GetApisResponseData, error)
	APIUpdate(ctx context.Context, req *apiv1.APIUpdateRequest) error
	APICreate(ctx context.Context, req *apiv1.APICreateRequest) error
	APIDelete(ctx context.Context, id uint) error
}

func NewAdminService(
	service *Service,
	adminRepository repository.AdminRepository,
	authService AuthService,
) AdminService {
	return &adminService{
		Service:         service,
		adminRepository: adminRepository,
		authService:     authService,
	}
}

type adminService struct {
	*Service
	adminRepository repository.AdminRepository
	// authService 用于改密 / 禁用 / 删除时调 RevokeAllUserSessions，
	// 让管理员的账号变更操作立即吊销该用户全部活跃 session。
	// 持有"业务接口"而非 *redis.Client，避免 wire 循环依赖与单测复杂度。
	authService AuthService
}

const dummyPasswordHash = "$2a$10$C6UzMDM.H6dfI/f/IKcEeO6DGw4ZSLiZUj2Ip7yUpfI2KI2Zg7W6e"

func pageQuery(p apiv1.Pagination) repository.PageQuery {
	p.Normalize()
	return repository.PageQuery{Page: p.Page, PageSize: p.PageSize}
}

func repositoryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrBadRequest):
		return apiv1.ErrBadRequest.WithCause(err)
	case errors.Is(err, repository.ErrConflict):
		return apiv1.ErrConflict.WithCause(err)
	case errors.Is(err, repository.ErrForbidden):
		return apiv1.ErrForbidden.WithCause(err)
	case errors.Is(err, repository.ErrNotFound):
		return apiv1.ErrNotFound.WithCause(err)
	case errors.Is(err, repository.ErrUsernameDuplicated):
		return apiv1.ErrUsernameAlreadyUse.WithCause(err)
	case errors.Is(err, repository.ErrRoleNameDuplicated):
		return apiv1.ErrRoleNameExists.WithCause(err)
	case errors.Is(err, repository.ErrRoleSIDDuplicated):
		return apiv1.ErrRoleSidExists.WithCause(err)
	default:
		return err
	}
}
