package repository

//go:generate go tool mockgen -destination=../../test/mocks/repository/admin.go rabc-go/internal/repository AdminRepository,AdminAuthRepository,AdminUserRepository,PermissionRepository,MenuRepository,RoleRepository,ApiRepository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"rabc-go/internal/model"
)

type AdminRepository interface {
	AdminAuthRepository
	AdminUserRepository
	PermissionRepository
	MenuRepository
	RoleRepository
	ApiRepository
}

type AdminAuthRepository interface {
	GetAdminUserByUsername(ctx context.Context, username string) (model.AdminUser, error)
	UpdateLastLogin(ctx context.Context, uid uint, at time.Time) error
}

type AdminUserRepository interface {
	GetAdminUsers(ctx context.Context, q AdminUserQuery) ([]model.AdminUser, int64, error)
	GetAdminUser(ctx context.Context, uid uint) (model.AdminUser, error)

	// Atomic 方法把"业务表写 + Casbin 策略写"包进同一个 GORM 事务。
	// 内部用临时 enforcer 绑 tx；commit 后调 r.e.LoadPolicy() 让全局 SyncedEnforcer 立即看到变更。
	AdminUserCreateAtomic(ctx context.Context, m *model.AdminUser, roles []string) error
	// AdminUserUpdateAtomic 接受 *[]string：nil 表示"未传 → 不动角色"，
	// 非 nil 表示"显式同步到该列表"（空切片含义为清空所有角色）。
	AdminUserUpdateAtomic(ctx context.Context, m *model.AdminUser, roles *[]string) error
	AdminUserDeleteAtomic(ctx context.Context, id uint) error
}

type PermissionRepository interface {
	GetUserPermissions(ctx context.Context, uid uint) ([][]string, error)
	GetUserRoles(ctx context.Context, uid uint) ([]string, error)
	GetRolePermissions(ctx context.Context, role string) ([][]string, error)
	UpdateRolePermission(ctx context.Context, role string, permissions map[string]struct{}) error
}

type MenuRepository interface {
	GetMenuList(ctx context.Context) ([]model.Menu, error)
	MenuCreate(ctx context.Context, m *model.Menu) error
	MenuUpdateAtomic(ctx context.Context, m *model.Menu) error
	MenuDeleteAtomic(ctx context.Context, id uint) error
}

type RoleRepository interface {
	GetRoles(ctx context.Context, q RoleQuery) ([]model.Role, int64, error)
	RoleUpdate(ctx context.Context, m *model.Role) error
	RoleCreateIfAbsent(ctx context.Context, m *model.Role) error
	RoleDeleteAtomic(ctx context.Context, id uint, sid string) error
	GetRole(ctx context.Context, id uint) (model.Role, error)
}

type ApiRepository interface {
	GetApis(ctx context.Context, q ApiQuery) ([]model.Api, int64, error)
	GetApiGroups(ctx context.Context) ([]string, error)
	ApiCreate(ctx context.Context, m *model.Api) error
	ApiUpdateAtomic(ctx context.Context, m *model.Api) error
	ApiDeleteAtomic(ctx context.Context, id uint) error
}

func NewAdminRepository(
	repository *Repository,
) AdminRepository {
	return &adminRepository{
		Repository: repository,
	}
}

// adminRepository 持有 RBAC 写路径专用的进程级互斥锁。
//
// 锁的语义：所有"业务表 + Casbin 策略"的复合写（Atomic 系列、
// UpdateRolePermission、UpdateUserRoles）必须串行执行。约束：
//   - 全局 SyncedEnforcer 启用了 StartAutoLoadPolicy 轮询，可能在 tx 提交后、
//     reloadPolicy 前把旧策略加载回来，破坏本进程"写完立即可见"语义；
//   - 临时 tx-bound enforcer 与全局 enforcer 各持有自己的 model 副本，
//     并发写会出现策略快照漂移；
//   - 单进程内串行能消除上述两点；多副本部署需另引入 DB advisory lock。
type adminRepository struct {
	*Repository
	rbacMu sync.Mutex
}

// newTxEnforcer 创建一个临时 Casbin enforcer，其 adapter 绑定传入的 GORM tx。
// 通过它做的所有 Casbin 写都参与该 tx，会跟着 commit/rollback。
//
// 两道关键屏障：
//  1. TurnOffAutoMigrate(tx) 跳过 adapter 的隐式 AutoMigrate + CREATE INDEX，
//     避免 MySQL 事务内 DDL 触发隐式 commit 破坏原子性。casbin_rule 表与
//     索引在应用启动时由全局 NewCasbinEnforcer 已建好。
//  2. NewCasbinModel() 重新 parse 得到独立 model 实例，不复用 r.e.GetModel()
//     （Casbin Model 是 map 值类型，多个并发 tx-bound enforcer 共享会竞态）。
func (r *adminRepository) newTxEnforcer(tx *gorm.DB) (casbin.IEnforcer, error) {
	gormadapter.TurnOffAutoMigrate(tx)
	a, err := gormadapter.NewAdapterByDB(tx)
	if err != nil {
		return nil, fmt.Errorf("init tx casbin adapter: %w", err)
	}
	m, err := NewCasbinModel()
	if err != nil {
		return nil, fmt.Errorf("init tx casbin model: %w", err)
	}
	e, err := casbin.NewEnforcer(m, a)
	if err != nil {
		return nil, fmt.Errorf("init tx casbin enforcer: %w", err)
	}
	return e, nil
}

// reloadPolicy 让全局 SyncedEnforcer 立即看到事务提交后的策略变更。
// 提交后的 reload 失败只记录日志；StartAutoLoadPolicy 会继续轮询兜底。
func (r *adminRepository) reloadPolicy(ctx context.Context) {
	if err := r.e.LoadPolicy(); err != nil {
		// 瞬时网络抖动：等 100ms 再试一次，多数抖动可恢复
		time.Sleep(100 * time.Millisecond)
		if err2 := r.e.LoadPolicy(); err2 != nil {
			r.logger.WithContext(ctx).Error(
				"重载 Casbin 策略失败",
				zap.NamedError("first_error", err),
				zap.NamedError("retry_error", err2),
			)
		}
	}
}

// updateUserRolesOn 在指定 enforcer 上把用户角色从 old → new 做 diff 同步。
func updateUserRolesOn(e casbin.IEnforcer, uid string, roles []string) error {
	if len(roles) == 0 {
		_, err := e.DeleteRolesForUser(uid)
		return err
	}
	old, err := e.GetRolesForUser(uid)
	if err != nil {
		return err
	}
	oldSet := make(map[string]struct{}, len(old))
	newSet := make(map[string]struct{}, len(roles))
	for _, v := range old {
		oldSet[v] = struct{}{}
	}
	for _, v := range roles {
		newSet[model.RoleSubject(v)] = struct{}{}
	}
	var addRoles, delRoles []string
	for k := range oldSet {
		if _, ok := newSet[k]; !ok {
			delRoles = append(delRoles, k)
		}
	}
	for k := range newSet {
		if _, ok := oldSet[k]; !ok {
			addRoles = append(addRoles, k)
		}
	}
	if len(addRoles) == 0 && len(delRoles) == 0 {
		return nil
	}
	for _, role := range delRoles {
		if _, err := e.DeleteRoleForUser(uid, role); err != nil {
			return err
		}
	}
	// IEnforcer 接口未暴露 AddRolesForUser（仅 *Enforcer/*SyncedEnforcer 有），
	// 用单条 AddRoleForUser 循环以保持接口适配。
	for _, role := range addRoles {
		if _, err := e.AddRoleForUser(uid, role); err != nil {
			return err
		}
	}
	return nil
}

// removePoliciesByObjectActOn 按 (obj, act) 维度清理策略。
// 用 RemoveFilteredPolicy 一步原子完成，避免 GetFilteredPolicy + RemovePolicies
// 两步式的快照不一致与 adapter 批量删除可能吞错的问题。
func removePoliciesByObjectActOn(e casbin.IEnforcer, obj, act string) error {
	_, err := e.RemoveFilteredPolicy(1, obj, act)
	return err
}

func ensureRolesExist(tx *gorm.DB, roles []string) error {
	roleSet := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		if strings.TrimSpace(role) == "" {
			return ErrBadRequest
		}
		roleSet[role] = struct{}{}
	}
	if len(roleSet) == 0 {
		return nil
	}
	sids := make([]string, 0, len(roleSet))
	for role := range roleSet {
		sids = append(sids, role)
	}
	var count int64
	if err := tx.Model(&model.Role{}).Where("sid IN ?", sids).Count(&count).Error; err != nil {
		return fmt.Errorf("count roles by sid: %w", err)
	}
	if count != int64(len(sids)) {
		return ErrBadRequest
	}
	return nil
}

func ensureRoleExists(tx *gorm.DB, role string) error {
	return ensureRolesExist(tx, []string{role})
}

type apiPermission struct {
	path   string
	method string
}

func ensurePermissionResourcesExist(tx *gorm.DB, permissions map[string]struct{}) error {
	apis := make(map[apiPermission]struct{})
	menus := make(map[string]struct{})
	for key := range permissions {
		resource, action, ok := strings.Cut(key, model.PermSep)
		if !ok || resource == "" || action == "" {
			return ErrBadRequest
		}
		switch {
		case strings.HasPrefix(resource, model.ApiResourcePrefix):
			apis[apiPermission{
				path:   strings.TrimPrefix(resource, model.ApiResourcePrefix),
				method: action,
			}] = struct{}{}
		case strings.HasPrefix(resource, model.MenuResourcePrefix):
			if action != "read" {
				return ErrBadRequest
			}
			menus[strings.TrimPrefix(resource, model.MenuResourcePrefix)] = struct{}{}
		default:
			return ErrBadRequest
		}
	}
	for api := range apis {
		var count int64
		if err := tx.Model(&model.Api{}).
			Where("path = ? AND method = ?", api.path, api.method).
			Count(&count).Error; err != nil {
			return fmt.Errorf("count api permission resource: %w", err)
		}
		if count == 0 {
			return ErrBadRequest
		}
	}
	for path := range menus {
		var count int64
		if err := tx.Model(&model.Menu{}).Where("path = ?", path).Count(&count).Error; err != nil {
			return fmt.Errorf("count menu permission resource: %w", err)
		}
		if count == 0 {
			return ErrBadRequest
		}
	}
	return nil
}

func ensureRowsAffected(tx *gorm.DB, result *gorm.DB, modelValue any, id uint) error {
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("%w: %v", ErrConflict, result.Error)
		}
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	var count int64
	if err := tx.Model(modelValue).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}
