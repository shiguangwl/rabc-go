package repository

//go:generate go tool mockgen -source=admin.go -destination=../../test/mocks/repository/admin.go

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"

	v1 "nunu-layout-admin/api/v1"
	"nunu-layout-admin/internal/model"
)

type AdminRepository interface {
	GetAdminUsers(ctx context.Context, req *v1.GetAdminUsersRequest) ([]model.AdminUser, int64, error)
	GetAdminUser(ctx context.Context, uid uint) (model.AdminUser, error)
	GetAdminUserByUsername(ctx context.Context, username string) (model.AdminUser, error)
	AdminUserUpdate(ctx context.Context, m *model.AdminUser) error
	AdminUserCreate(ctx context.Context, m *model.AdminUser) error
	AdminUserDelete(ctx context.Context, id uint) error

	// Atomic 方法把"业务表写 + Casbin 策略写"包进同一个 GORM 事务。
	// 内部用临时 enforcer 绑 tx；commit 后调 r.e.LoadPolicy() 让全局 SyncedEnforcer 立即看到变更。
	AdminUserCreateAtomic(ctx context.Context, m *model.AdminUser, roles []string) error
	// AdminUserUpdateAtomic 接受 *[]string：nil 表示"未传 → 不动角色"，
	// 非 nil 表示"显式同步到该列表"（空切片含义为清空所有角色）。
	AdminUserUpdateAtomic(ctx context.Context, m *model.AdminUser, roles *[]string) error
	AdminUserDeleteAtomic(ctx context.Context, id uint) error
	RoleDeleteAtomic(ctx context.Context, id uint, sid string) error
	MenuUpdateAtomic(ctx context.Context, m *model.Menu, oldPath string) error
	MenuDeleteAtomic(ctx context.Context, id uint, oldPath string) error
	ApiUpdateAtomic(ctx context.Context, m *model.Api, oldPath, oldMethod string) error
	ApiDeleteAtomic(ctx context.Context, id uint, oldPath, oldMethod string) error

	GetUserPermissions(ctx context.Context, uid uint) ([][]string, error)
	GetUserRoles(ctx context.Context, uid uint) ([]string, error)
	GetRolePermissions(ctx context.Context, role string) ([][]string, error)
	UpdateRolePermission(ctx context.Context, role string, permissions map[string]struct{}) error
	UpdateUserRoles(ctx context.Context, uid uint, roles []string) error
	DeleteUserRoles(ctx context.Context, uid uint) error

	GetMenuList(ctx context.Context) ([]model.Menu, error)
	GetMenu(ctx context.Context, id uint) (model.Menu, error)
	MenuUpdate(ctx context.Context, m *model.Menu) error
	MenuCreate(ctx context.Context, m *model.Menu) error
	MenuDelete(ctx context.Context, id uint) error

	GetRoles(ctx context.Context, req *v1.GetRoleListRequest) ([]model.Role, int64, error)
	RoleUpdate(ctx context.Context, m *model.Role) error
	RoleCreate(ctx context.Context, m *model.Role) error
	RoleCreateIfAbsent(ctx context.Context, m *model.Role) error
	RoleDelete(ctx context.Context, id uint) error
	CasbinRoleDelete(ctx context.Context, role string) error
	GetRole(ctx context.Context, id uint) (model.Role, error)

	GetApis(ctx context.Context, req *v1.GetApisRequest) ([]model.Api, int64, error)
	GetApi(ctx context.Context, id uint) (model.Api, error)
	GetApiGroups(ctx context.Context) ([]string, error)
	ApiUpdate(ctx context.Context, m *model.Api) error
	ApiCreate(ctx context.Context, m *model.Api) error
	ApiDelete(ctx context.Context, id uint) error
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
// UpdateRolePermission、UpdateUserRoles）必须串行执行。原因：
//   - 全局 SyncedEnforcer 启用了 StartAutoLoadPolicy 轮询，可能在 tx 提交后、
//     reloadPolicy 之前把旧策略加载回来，与本进程的"写完立即可见"语义打架；
//   - 临时 tx-bound enforcer 与全局 enforcer 各持有自己的 model 副本，
//     并发写会出现策略快照漂移；
//   - 单进程内串行能消除上述两点；多副本部署需另引入 DB advisory lock。
type adminRepository struct {
	*Repository
	rbacMu sync.Mutex
}

func (r *adminRepository) CasbinRoleDelete(ctx context.Context, role string) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	_, err := r.e.DeleteRole(role)
	return err
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

// reloadPolicy 让全局 SyncedEnforcer 立即看到事务提交后的策略变更，
// 避免等 StartAutoLoadPolicy 的 10 秒轮询窗口。
//
// 关键语义：tx 已 commit 即视为业务成功；reload 失败仅记 ERROR 让 SRE 关注，
// 不向上传错——否则前端会以为整个操作失败并重试，重试会落到"DB/Casbin
// 已写入但当前进程缓存未更新"的奇怪状态，更糟。
// 兜底：StartAutoLoadPolicy 10 秒内会自动同步策略。
func (r *adminRepository) reloadPolicy(ctx context.Context) {
	if err := r.e.LoadPolicy(); err != nil {
		// 瞬时网络抖动：等 100ms 再试一次，多数抖动可恢复
		time.Sleep(100 * time.Millisecond)
		if err2 := r.e.LoadPolicy(); err2 != nil {
			r.logger.WithContext(ctx).Error(
				"reload casbin policy after tx commit failed (retry exhausted); will be picked up by autoload within 10s",
				zap.NamedError("first", err),
				zap.NamedError("retry", err2),
			)
		}
	}
}

// updateUserRolesOn 在指定 enforcer 上把用户角色从 old → new 做 diff 同步。
// 抽出来是为了让全局 enforcer（UpdateUserRoles）和 tx-bound enforcer（Atomic 系列）共用。
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
		newSet[v] = struct{}{}
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

func ensureRowsAffected(tx *gorm.DB, result *gorm.DB, modelValue any, id uint) error {
	if result.Error != nil {
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
		return v1.ErrNotFound
	}
	return nil
}

func (r *adminRepository) AdminUserCreateAtomic(ctx context.Context, m *model.AdminUser, roles []string) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			// username 唯一索引冲突翻译为业务 sentinel，避免裸驱动错被 WriteResponse 兜底成 500，
			// 让前端按业务码精确提示"用户名已被占用"。
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return v1.ErrUsernameAlreadyUse.WithCause(err)
			}
			return err
		}
		if len(roles) == 0 {
			return nil
		}
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		return updateUserRolesOn(e, strconv.FormatUint(uint64(m.ID), 10), roles)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

// AdminUserUpdateAtomic 按"非空字段才写"语义更新业务表 + 同步 Casbin 角色。
//
// 设计：
//   - 用 map 而非 struct：struct 模式会跳过零值字段，使得真要"显式覆盖"也落不到库。
//   - 又仅纳入非空字段：避免前端漏传 email/phone 时静默清空已有值。
//     需要主动清空的业务（暂无）应在 API 层用指针类型区分"未传"vs"空串"，
//     再由 service 层显式标记为 NULL 写入。
//   - password 列空值语义是"不修改密码"，与上述策略天然一致；同时避免 service 层
//     事务外 read-modify-write 被并发覆盖丢密码（详见 service.AdminUserUpdate 注释）。
func (r *adminRepository) AdminUserUpdateAtomic(ctx context.Context, m *model.AdminUser, roles *[]string) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	updates := map[string]any{}
	if m.Username != "" {
		updates["username"] = m.Username
	}
	if m.Nickname != "" {
		updates["nickname"] = m.Nickname
	}
	if m.Email != "" {
		updates["email"] = m.Email
	}
	if m.Phone != "" {
		updates["phone"] = m.Phone
	}
	if m.Password != "" {
		updates["password"] = m.Password
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			result := tx.Model(&model.AdminUser{}).Where("id = ?", m.ID).Updates(updates)
			if result.Error != nil {
				if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
					return v1.ErrUsernameAlreadyUse.WithCause(result.Error)
				}
				return result.Error
			}
			if err := ensureRowsAffected(tx, result, &model.AdminUser{}, m.ID); err != nil {
				return err
			}
		} else if roles != nil {
			var count int64
			if err := tx.Model(&model.AdminUser{}).Where("id = ?", m.ID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return v1.ErrNotFound
			}
		}
		// roles 为 nil 表示前端未传该字段，跳过角色同步避免误清空。
		// 显式传空数组（*roles == []string{}）会走 updateUserRolesOn 的 len==0 分支，
		// 语义为"清空全部角色"，由前端业务自行确认。
		if roles == nil {
			return nil
		}
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		return updateUserRolesOn(e, strconv.FormatUint(uint64(m.ID), 10), *roles)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) AdminUserDeleteAtomic(ctx context.Context, id uint) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		// 撤权先：先清角色绑定再删 DB 行。tx 失败两者一起回滚；不会出现"用户被删但权限残留"。
		if _, err := e.DeleteRolesForUser(strconv.FormatUint(uint64(id), 10)); err != nil {
			return err
		}
		return ensureRowsAffected(tx, tx.Where("id = ?", id).Delete(&model.AdminUser{}), &model.AdminUser{}, id)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) RoleDeleteAtomic(ctx context.Context, id uint, sid string) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		// 撤权先：先清 Casbin 策略再删 DB 行；tx 失败一起回滚。
		if _, err := e.DeleteRole(sid); err != nil {
			return err
		}
		return ensureRowsAffected(tx, tx.Where("id = ?", id).Delete(&model.Role{}), &model.Role{}, id)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

// ApiUpdateAtomic 用 map 触发更新（理由同 AdminUserUpdateAtomic）。
// path/method 变更时事务内同步清旧 Casbin 策略。
func (r *adminRepository) ApiUpdateAtomic(ctx context.Context, m *model.Api, oldPath, oldMethod string) error {
	pathChanged := oldPath != m.Path || oldMethod != m.Method
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	updates := map[string]any{
		"group_name": m.Group,
		"name":       m.Name,
		"path":       m.Path,
		"method":     m.Method,
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureRowsAffected(tx, tx.Model(&model.Api{}).Where("id = ?", m.ID).Updates(updates), &model.Api{}, m.ID); err != nil {
			return err
		}
		if !pathChanged {
			return nil
		}
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		return removePoliciesByObjectActOn(e, model.ApiResourcePrefix+oldPath, oldMethod)
	}); err != nil {
		return err
	}
	if pathChanged {
		r.reloadPolicy(ctx)
	}
	return nil
}

func (r *adminRepository) MenuUpdateAtomic(ctx context.Context, m *model.Menu, oldPath string) error {
	pathChanged := oldPath != m.Path
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	updates := map[string]any{
		"component":    m.Component,
		"icon":         m.Icon,
		"keep_alive":   m.KeepAlive,
		"hide_in_menu": m.HideInMenu,
		"locale":       m.Locale,
		"weight":       m.Weight,
		"name":         m.Name,
		"parent_id":    m.ParentID,
		"path":         m.Path,
		"redirect":     m.Redirect,
		"title":        m.Title,
		"url":          m.URL,
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureRowsAffected(tx, tx.Model(&model.Menu{}).Where("id = ?", m.ID).Updates(updates), &model.Menu{}, m.ID); err != nil {
			return err
		}
		if !pathChanged {
			return nil
		}
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		return removePoliciesByObjectActOn(e, model.MenuResourcePrefix+oldPath, "read")
	}); err != nil {
		return err
	}
	if pathChanged {
		r.reloadPolicy(ctx)
	}
	return nil
}

func (r *adminRepository) ApiDeleteAtomic(ctx context.Context, id uint, oldPath, oldMethod string) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		// 撤权先：清 Casbin 策略后再删 DB 行
		if err := removePoliciesByObjectActOn(e, model.ApiResourcePrefix+oldPath, oldMethod); err != nil {
			return err
		}
		return ensureRowsAffected(tx, tx.Where("id = ?", id).Delete(&model.Api{}), &model.Api{}, id)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) MenuDeleteAtomic(ctx context.Context, id uint, oldPath string) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		if err := removePoliciesByObjectActOn(e, model.MenuResourcePrefix+oldPath, "read"); err != nil {
			return err
		}
		return ensureRowsAffected(tx, tx.Where("id = ?", id).Delete(&model.Menu{}), &model.Menu{}, id)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) GetRole(ctx context.Context, id uint) (model.Role, error) {
	m := model.Role{}
	return m, r.DB(ctx).Where("id = ?", id).First(&m).Error
}
func (r *adminRepository) DeleteUserRoles(ctx context.Context, uid uint) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	_, err := r.e.DeleteRolesForUser(strconv.FormatUint(uint64(uid), 10))
	return err
}

// UpdateUserRoles 把全局 enforcer 上指定用户的角色同步到 roles 列表。
// 实际算法在 updateUserRolesOn 内，与 Atomic 系列共用。
func (r *adminRepository) UpdateUserRoles(ctx context.Context, uid uint, roles []string) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	return updateUserRolesOn(r.e, strconv.FormatUint(uint64(uid), 10), roles)
}

func (r *adminRepository) GetAdminUserByUsername(ctx context.Context, username string) (model.AdminUser, error) {
	m := model.AdminUser{}
	return m, r.DB(ctx).Where("username = ?", username).First(&m).Error
}

func (r *adminRepository) GetAdminUsers(ctx context.Context, req *v1.GetAdminUsersRequest) ([]model.AdminUser, int64, error) {
	var list []model.AdminUser
	var total int64
	scope := r.DB(ctx).Model(&model.AdminUser{})
	if req.Username != "" {
		scope = scope.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Nickname != "" {
		scope = scope.Where("nickname LIKE ?", "%"+req.Nickname+"%")
	}
	if req.Email != "" {
		scope = scope.Where("email LIKE ?", "%"+req.Email+"%")
	}
	if req.Phone != "" {
		scope = scope.Where("phone LIKE ?", "%"+req.Phone+"%")
	}
	if err := scope.Count(&total).Error; err != nil {
		return nil, total, err
	}
	if err := scope.Offset(req.Offset()).Limit(req.Limit()).Order("id DESC").Find(&list).Error; err != nil {
		return nil, total, err
	}
	return list, total, nil
}

func (r *adminRepository) GetAdminUser(ctx context.Context, uid uint) (model.AdminUser, error) {
	m := model.AdminUser{}
	return m, r.DB(ctx).Where("id = ?", uid).First(&m).Error
}

func (r *adminRepository) AdminUserUpdate(ctx context.Context, m *model.AdminUser) error {
	return r.DB(ctx).Where("id = ?", m.ID).Updates(m).Error
}

func (r *adminRepository) AdminUserCreate(ctx context.Context, m *model.AdminUser) error {
	return r.DB(ctx).Create(m).Error
}

func (r *adminRepository) AdminUserDelete(ctx context.Context, id uint) error {
	return r.DB(ctx).Where("id = ?", id).Delete(&model.AdminUser{}).Error
}

// UpdateRolePermission 把指定角色的权限集合同步成 newPermSet。
//
// 走 tx-bound enforcer：旧实现直接操作全局 r.e，删 N 条后 add 失败时已删的回不去——
// 与 *Atomic 系列同样的部分失败窗口。这里包进 r.db.Transaction，
// 由 newTxEnforcer 在事务里完成 diff/删/加，commit 后 reload 全局 enforcer。
func (r *adminRepository) UpdateRolePermission(ctx context.Context, role string, newPermSet map[string]struct{}) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		oldPermissions, err := e.GetPermissionsForUser(role)
		if err != nil {
			return err
		}
		oldPermSet := make(map[string]struct{}, len(oldPermissions))
		for _, perm := range oldPermissions {
			if len(perm) == 3 {
				oldPermSet[strings.Join([]string{perm[1], perm[2]}, model.PermSep)] = struct{}{}
			}
		}
		var removePermissions, addPermissions [][]string
		for key := range oldPermSet {
			if _, ok := newPermSet[key]; !ok {
				removePermissions = append(removePermissions, strings.Split(key, model.PermSep))
			}
		}
		for key := range newPermSet {
			if _, ok := oldPermSet[key]; !ok {
				addPermissions = append(addPermissions, strings.Split(key, model.PermSep))
			}
		}
		// IEnforcer 接口未暴露 AddPermissionsForUser（仅 *Enforcer 有），
		// 用单条 AddPermissionForUser 循环以保持接口适配。
		for _, perm := range removePermissions {
			if _, err := e.DeletePermissionForUser(role, perm...); err != nil {
				return fmt.Errorf("remove permission %v: %w", perm, err)
			}
		}
		for _, perm := range addPermissions {
			if _, err := e.AddPermissionForUser(role, perm...); err != nil {
				return fmt.Errorf("add permission %v: %w", perm, err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) GetApiGroups(ctx context.Context) ([]string, error) {
	res := make([]string, 0)
	if err := r.DB(ctx).Model(&model.Api{}).Distinct().Order("group_name ASC").Pluck("group_name", &res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (r *adminRepository) GetApis(ctx context.Context, req *v1.GetApisRequest) ([]model.Api, int64, error) {
	var list []model.Api
	var total int64
	scope := r.DB(ctx).Model(&model.Api{})
	if req.Name != "" {
		scope = scope.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Group != "" {
		scope = scope.Where("group_name LIKE ?", "%"+req.Group+"%")
	}
	if req.Path != "" {
		scope = scope.Where("path LIKE ?", "%"+req.Path+"%")
	}
	if req.Method != "" {
		scope = scope.Where("method = ?", req.Method)
	}
	if err := scope.Count(&total).Error; err != nil {
		return nil, total, err
	}
	if err := scope.Offset(req.Offset()).Limit(req.Limit()).Order("group_name ASC").Find(&list).Error; err != nil {
		return nil, total, err
	}
	return list, total, nil
}

func (r *adminRepository) ApiUpdate(ctx context.Context, m *model.Api) error {
	result := r.DB(ctx).Where("id = ?", m.ID).Updates(m)
	return ensureRowsAffected(r.DB(ctx), result, &model.Api{}, m.ID)
}

func (r *adminRepository) ApiCreate(ctx context.Context, m *model.Api) error {
	return r.DB(ctx).Create(m).Error
}

func (r *adminRepository) ApiDelete(ctx context.Context, id uint) error {
	result := r.DB(ctx).Where("id = ?", id).Delete(&model.Api{})
	return ensureRowsAffected(r.DB(ctx), result, &model.Api{}, id)
}

func (r *adminRepository) GetApi(ctx context.Context, id uint) (model.Api, error) {
	m := model.Api{}
	return m, r.DB(ctx).Where("id = ?", id).First(&m).Error
}

func (r *adminRepository) GetUserPermissions(ctx context.Context, uid uint) ([][]string, error) {
	return r.e.GetImplicitPermissionsForUser(strconv.FormatUint(uint64(uid), 10))

}
func (r *adminRepository) GetRolePermissions(ctx context.Context, role string) ([][]string, error) {
	return r.e.GetPermissionsForUser(role)
}
func (r *adminRepository) GetUserRoles(ctx context.Context, uid uint) ([]string, error) {
	return r.e.GetRolesForUser(strconv.FormatUint(uint64(uid), 10))
}
func (r *adminRepository) MenuUpdate(ctx context.Context, m *model.Menu) error {
	result := r.DB(ctx).Where("id = ?", m.ID).Updates(m)
	return ensureRowsAffected(r.DB(ctx), result, &model.Menu{}, m.ID)
}

func (r *adminRepository) MenuCreate(ctx context.Context, m *model.Menu) error {
	return r.DB(ctx).Create(m).Error
}

func (r *adminRepository) MenuDelete(ctx context.Context, id uint) error {
	result := r.DB(ctx).Where("id = ?", id).Delete(&model.Menu{})
	return ensureRowsAffected(r.DB(ctx), result, &model.Menu{}, id)
}

func (r *adminRepository) GetMenuList(ctx context.Context) ([]model.Menu, error) {
	var menuList []model.Menu
	if err := r.DB(ctx).Order("weight DESC").Find(&menuList).Error; err != nil {
		return nil, err
	}
	return menuList, nil
}

func (r *adminRepository) GetMenu(ctx context.Context, id uint) (model.Menu, error) {
	m := model.Menu{}
	return m, r.DB(ctx).Where("id = ?", id).First(&m).Error
}

func (r *adminRepository) RoleUpdate(ctx context.Context, m *model.Role) error {
	result := r.DB(ctx).Model(&model.Role{}).Where("id = ?", m.ID).UpdateColumn("name", m.Name)
	return ensureRowsAffected(r.DB(ctx), result, &model.Role{}, m.ID)
}

func (r *adminRepository) RoleCreate(ctx context.Context, m *model.Role) error {
	return r.DB(ctx).Create(m).Error
}

// RoleCreateIfAbsent 直接 Create；唯一约束冲突时通过反查精确翻译为
// ErrRoleSidExists / ErrRoleNameExists，避免给前端返回裸 driver 错。
//
// 取舍（MySQL 语义）：INSERT IGNORE / OnConflict.DoNothing 会吞掉所有 unique 索引冲突且
// 不告知是哪一列；旧实现据此把任何冲突一律映射为 sid 冲突，会误导前端。
// 这里改为先 INSERT，1062 后再用 name / sid 反查（仅冲突路径多一次查询），
// 业务码精确度优先于一次额外查询。极小概率的 TOCTOU（冲突行被并发删除）下
// 回退到 ErrRoleSidExists（保守兜底）由上层兜底成 409，避免穿透成 500。
// PostgreSQL 的 ON CONFLICT DO NOTHING 可指定 columns，切库时可重新评估。
func (r *adminRepository) RoleCreateIfAbsent(ctx context.Context, m *model.Role) error {
	err := r.DB(ctx).Create(m).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return err
	}
	var existing model.Role
	// .Unscoped() 确保能查到被软删除但仍占着 unique key 的行；
	// .Limit(1) 避免拿整表后 First，同时配合 Unscoped 防止误匹配。
	if e := r.db.WithContext(ctx).Unscoped().Where("name = ?", m.Name).Limit(1).First(&existing).Error; e == nil {
		return v1.ErrRoleNameExists.WithCause(err)
	}
	if e := r.db.WithContext(ctx).Unscoped().Where("sid = ?", m.Sid).Limit(1).First(&existing).Error; e == nil {
		return v1.ErrRoleSidExists.WithCause(err)
	}
	// 反查均未命中：冲突行已被并发删除（极小概率 TOCTOU）。
	// 已确认 err 是 ErrDuplicatedKey，保守兜底为 ErrRoleSidExists 而不让 driver 错
	// 穿透到 WriteResponse 落成 500——前端拿到 409 后重试一次大概率成功。
	return v1.ErrRoleSidExists.WithCause(err)
}

func (r *adminRepository) RoleDelete(ctx context.Context, id uint) error {
	result := r.DB(ctx).Where("id = ?", id).Delete(&model.Role{})
	return ensureRowsAffected(r.DB(ctx), result, &model.Role{}, id)
}

func (r *adminRepository) GetRoles(ctx context.Context, req *v1.GetRoleListRequest) ([]model.Role, int64, error) {
	var list []model.Role
	var total int64
	scope := r.DB(ctx).Model(&model.Role{})
	if req.Name != "" {
		scope = scope.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Sid != "" {
		scope = scope.Where("sid = ?", req.Sid)
	}
	if err := scope.Count(&total).Error; err != nil {
		return nil, total, err
	}
	if err := scope.Offset(req.Offset()).Limit(req.Limit()).Find(&list).Error; err != nil {
		return nil, total, err
	}
	return list, total, nil
}
