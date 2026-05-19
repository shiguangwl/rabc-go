package casbinkit

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
)

// EnsureRoles 在事务内校验 roles 中的每个 sid 都存在于 roles 表。
//
// 契约：返回 ErrBadRequest 表示"客户端输入了未知/空 sid"，调用方一般直接 400；
// 包含空字符串与任一 sid 不存在共用同一错误，避免泄漏存在性。
func EnsureRoles(tx *gorm.DB, roles []string) error {
	roleSet := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		if strings.TrimSpace(role) == "" {
			return apiv1.ErrBadRequest
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
		return apiv1.ErrBadRequest
	}
	return nil
}

func EnsureRole(tx *gorm.DB, role string) error {
	return EnsureRoles(tx, []string{role})
}

type apiPermission struct {
	path   string
	method string
}

// EnsurePermissionResources 校验权限串集合对应的 API / Menu 资源在表内全部存在。
//
// 契约：格式非法、未知前缀、Menu 非 read action、资源行不存在 统一返回 ErrBadRequest。
// 不返回 NotFound 是为了避免客户端探测"哪些资源已存在"。
func EnsurePermissionResources(tx *gorm.DB, permissions map[string]struct{}) error {
	apis := make(map[apiPermission]struct{})
	menus := make(map[string]struct{})
	for key := range permissions {
		resource, action, ok := strings.Cut(key, model.PermSep)
		if !ok || resource == "" || action == "" {
			return apiv1.ErrBadRequest
		}
		switch {
		case strings.HasPrefix(resource, model.APIResourcePrefix):
			apis[apiPermission{
				path:   strings.TrimPrefix(resource, model.APIResourcePrefix),
				method: action,
			}] = struct{}{}
		case strings.HasPrefix(resource, model.MenuResourcePrefix):
			if action != "read" {
				return apiv1.ErrBadRequest
			}
			menus[strings.TrimPrefix(resource, model.MenuResourcePrefix)] = struct{}{}
		default:
			return apiv1.ErrBadRequest
		}
	}
	for api := range apis {
		var count int64
		if err := tx.Model(&model.API{}).
			Where("path = ? AND method = ?", api.path, api.method).
			Count(&count).Error; err != nil {
			return fmt.Errorf("count api permission resource: %w", err)
		}
		if count == 0 {
			return apiv1.ErrBadRequest
		}
	}
	for path := range menus {
		var count int64
		if err := tx.Model(&model.Menu{}).Where("path = ?", path).Count(&count).Error; err != nil {
			return fmt.Errorf("count menu permission resource: %w", err)
		}
		if count == 0 {
			return apiv1.ErrBadRequest
		}
	}
	return nil
}

// EnsureRowsAffected 区分三种写入结果：成功 / 主键不存在 / 唯一键冲突。
//
// 直接用 result.RowsAffected==0 判 NotFound 会误报——若行存在但 UPDATE 字段
// 等于原值，MySQL 也返回 0。本函数在 RowsAffected==0 时回查一次行存在性，
// 让上层错误码语义稳定（ErrNotFound vs nil）。唯一键冲突包装为 ErrConflict。
func EnsureRowsAffected(tx, result *gorm.DB, modelValue any, id uint) error {
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("%w: %w", apiv1.ErrConflict, result.Error)
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
		return apiv1.ErrNotFound
	}
	return nil
}
