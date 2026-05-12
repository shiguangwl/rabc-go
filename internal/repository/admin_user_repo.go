package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

	"rabc-go/internal/model"
)

func (r *adminRepository) AdminUserCreateAtomic(ctx context.Context, m *model.AdminUser, roles []string) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return fmt.Errorf("%w: %v", ErrUsernameDuplicated, err)
			}
			return err
		}
		if len(roles) == 0 {
			return nil
		}
		if err := ensureRolesExist(tx, roles); err != nil {
			return err
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

// AdminUserUpdateAtomic 按"非空字段才写"语义更新用户资料。
// 当前请求模型不支持显式清空字符串字段；password 空值表示不修改密码。
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
					return fmt.Errorf("%w: %v", ErrUsernameDuplicated, result.Error)
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
				return ErrNotFound
			}
		}
		if roles == nil {
			return nil
		}
		if err := ensureRolesExist(tx, *roles); err != nil {
			return err
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
	if strconv.FormatUint(uint64(id), 10) == model.AdminUserID {
		return ErrForbidden
	}
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
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

func (r *adminRepository) GetAdminUserByUsername(ctx context.Context, username string) (model.AdminUser, error) {
	m := model.AdminUser{}
	return m, r.DB(ctx).Where("username = ?", username).First(&m).Error
}

func (r *adminRepository) GetAdminUsers(ctx context.Context, q AdminUserQuery) ([]model.AdminUser, int64, error) {
	var list []model.AdminUser
	var total int64
	scope := r.DB(ctx).Model(&model.AdminUser{})
	if q.Username != "" {
		scope = scope.Where("username LIKE ?", "%"+q.Username+"%")
	}
	if q.Nickname != "" {
		scope = scope.Where("nickname LIKE ?", "%"+q.Nickname+"%")
	}
	if q.Email != "" {
		scope = scope.Where("email LIKE ?", "%"+q.Email+"%")
	}
	if q.Phone != "" {
		scope = scope.Where("phone LIKE ?", "%"+q.Phone+"%")
	}
	if err := scope.Count(&total).Error; err != nil {
		return nil, total, err
	}
	if err := scope.Offset(q.Offset()).Limit(q.Limit()).Order("id DESC").Find(&list).Error; err != nil {
		return nil, total, err
	}
	return list, total, nil
}

func (r *adminRepository) GetAdminUser(ctx context.Context, uid uint) (model.AdminUser, error) {
	m := model.AdminUser{}
	return m, r.DB(ctx).Where("id = ?", uid).First(&m).Error
}

// UpdateLastLogin 只写 last_login_at 单列，避开 Updates(struct) 误清空零值的坑。
func (r *adminRepository) UpdateLastLogin(ctx context.Context, uid uint, at time.Time) error {
	return r.DB(ctx).Model(&model.AdminUser{}).
		Where("id = ?", uid).
		Update("last_login_at", at).Error
}
