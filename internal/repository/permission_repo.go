package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"rabc-go/internal/model"
)

func (r *adminRepository) RoleDeleteAtomic(ctx context.Context, id uint, sid string) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		// 撤权先：先清 Casbin 策略再删 DB 行；tx 失败一起回滚。
		if _, err := e.DeleteRole(model.RoleSubject(sid)); err != nil {
			return err
		}
		return ensureRowsAffected(tx, tx.Where("id = ?", id).Delete(&model.Role{}), &model.Role{}, id)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) UpdateRolePermission(ctx context.Context, role string, newPermSet map[string]struct{}) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureRoleExists(tx, role); err != nil {
			return err
		}
		if err := ensurePermissionResourcesExist(tx, newPermSet); err != nil {
			return err
		}
		e, err := r.newTxEnforcer(tx)
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
		// IEnforcer 接口未暴露 AddPermissionsForUser（仅 *Enforcer 有），
		// 用单条 AddPermissionForUser 循环以保持接口适配。
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
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) GetUserPermissions(_ context.Context, uid uint) ([][]string, error) {
	return r.e.GetImplicitPermissionsForUser(strconv.FormatUint(uint64(uid), 10))
}

func (r *adminRepository) GetRolePermissions(_ context.Context, role string) ([][]string, error) {
	return r.e.GetPermissionsForUser(model.RoleSubject(role))
}

func (r *adminRepository) GetUserRoles(_ context.Context, uid uint) ([]string, error) {
	roles, err := r.e.GetRolesForUser(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return nil, err
	}
	for i, role := range roles {
		roles[i] = model.RoleSID(role)
	}
	return roles, nil
}
