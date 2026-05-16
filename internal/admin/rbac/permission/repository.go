// Package permission 提供 RBAC 角色权限的查询与编排。
//
// 关键不变量：UpdateRolePermission 必须在同一 DB 事务内同步业务表与 Casbin
// 策略；提交后必须调用 casbinkit.Reload，否则全局 SyncedEnforcer 会在 AutoLoad
// 轮询窗口内返回旧策略。
package permission

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"rabc-go/internal/admin/rbac/casbinkit"
	"rabc-go/internal/model"
	"rabc-go/pkg/log"
)

type Repo struct {
	db     *gorm.DB
	e      *casbin.SyncedEnforcer
	logger *log.Logger
	mu     *casbinkit.RBACMu
}

func NewRepo(db *gorm.DB, e *casbin.SyncedEnforcer, logger *log.Logger, mu *casbinkit.RBACMu) *Repo {
	return &Repo{db: db, e: e, logger: logger, mu: mu}
}

func (r *Repo) GetUserPermissions(_ context.Context, uid uint) ([][]string, error) {
	return r.e.GetImplicitPermissionsForUser(strconv.FormatUint(uint64(uid), 10))
}

func (r *Repo) GetRolePermissions(_ context.Context, role string) ([][]string, error) {
	return r.e.GetPermissionsForUser(model.RoleSubject(role))
}

func (r *Repo) UpdateRolePermission(ctx context.Context, role string, newPermSet map[string]struct{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := casbinkit.EnsureRole(tx, role); err != nil {
			return err
		}
		if err := casbinkit.EnsurePermissionResources(tx, newPermSet); err != nil {
			return err
		}
		e, err := casbinkit.NewTxEnforcer(tx)
		if err != nil {
			return err
		}
		roleSubject := model.RoleSubject(role)
		oldPermissions, err := e.GetPermissionsForUser(roleSubject)
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
		// IEnforcer 未暴露批量 AddPermissionsForUser，逐条调用是接口约束而非选择。
		for _, perm := range removePermissions {
			if _, err := e.DeletePermissionForUser(roleSubject, perm...); err != nil {
				return fmt.Errorf("remove permission %v: %w", perm, err)
			}
		}
		for _, perm := range addPermissions {
			if _, err := e.AddPermissionForUser(roleSubject, perm...); err != nil {
				return fmt.Errorf("add permission %v: %w", perm, err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	casbinkit.Reload(ctx, r.e, r.logger)
	return nil
}
