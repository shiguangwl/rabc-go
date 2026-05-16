package user

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/admin/rbac/casbinkit"
	"rabc-go/internal/model"
	"rabc-go/pkg/log"
)

// Repo 中带 Atomic 后缀的方法承诺：DB 写与 Casbin 策略变更在同一事务内完成，
// 任一步失败整体回滚；调用方不得在事务外自行拼装这两步。
type Repo struct {
	db     *gorm.DB
	e      *casbin.SyncedEnforcer
	logger *log.Logger
	mu     *casbinkit.RBACMu
}

func NewRepo(db *gorm.DB, e *casbin.SyncedEnforcer, logger *log.Logger, mu *casbinkit.RBACMu) *Repo {
	return &Repo{db: db, e: e, logger: logger, mu: mu}
}

func (r *Repo) GetAdminUserByUsername(ctx context.Context, username string) (model.AdminUser, error) {
	m := model.AdminUser{}
	return m, r.db.WithContext(ctx).Where("username = ?", username).First(&m).Error
}

// UpdateLastLogin 必须用 Update 单列写，不能用 Updates(struct)：后者会把零值字段一并清空。
func (r *Repo) UpdateLastLogin(ctx context.Context, uid uint, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.AdminUser{}).
		Where("id = ?", uid).
		Update("last_login_at", at).Error
}

func (r *Repo) GetAdminUsers(ctx context.Context, q Query) ([]model.AdminUser, int64, error) {
	var list []model.AdminUser
	var total int64
	scope := r.db.WithContext(ctx).Model(&model.AdminUser{})
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

func (r *Repo) GetAdminUser(ctx context.Context, uid uint) (model.AdminUser, error) {
	m := model.AdminUser{}
	return m, r.db.WithContext(ctx).Where("id = ?", uid).First(&m).Error
}

func (r *Repo) GetUserRoles(_ context.Context, uid uint) ([]string, error) {
	roles, err := r.e.GetRolesForUser(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return nil, err
	}
	for i, role := range roles {
		roles[i] = model.RoleSID(role)
	}
	return roles, nil
}

func (r *Repo) AdminUserCreateAtomic(ctx context.Context, m *model.AdminUser, roles []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return fmt.Errorf("%w: %w", apiv1.ErrUsernameAlreadyUse, err)
			}
			return err
		}
		if len(roles) == 0 {
			return nil
		}
		if err := casbinkit.EnsureRoles(tx, roles); err != nil {
			return err
		}
		e, err := casbinkit.NewTxEnforcer(tx)
		if err != nil {
			return err
		}
		return casbinkit.UpdateUserRolesOn(e, strconv.FormatUint(uint64(m.ID), 10), roles)
	}); err != nil {
		return err
	}
	casbinkit.Reload(ctx, r.e, r.logger)
	return nil
}

// AdminUserUpdateAtomic 按"非空字段才写"语义更新：当前请求模型无法表达"显式清空字符串"，
// 因此空字符串一律视作"未传"。Password 同理：空 = 不改密码。
func (r *Repo) AdminUserUpdateAtomic(ctx context.Context, m *model.AdminUser, roles *[]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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
					return fmt.Errorf("%w: %w", apiv1.ErrUsernameAlreadyUse, result.Error)
				}
				return result.Error
			}
			if err := casbinkit.EnsureRowsAffected(tx, result, &model.AdminUser{}, m.ID); err != nil {
				return err
			}
		} else if roles != nil {
			var count int64
			if err := tx.Model(&model.AdminUser{}).Where("id = ?", m.ID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return apiv1.ErrNotFound
			}
		}
		if roles == nil {
			return nil
		}
		if err := casbinkit.EnsureRoles(tx, *roles); err != nil {
			return err
		}
		e, err := casbinkit.NewTxEnforcer(tx)
		if err != nil {
			return err
		}
		return casbinkit.UpdateUserRolesOn(e, strconv.FormatUint(uint64(m.ID), 10), *roles)
	}); err != nil {
		return err
	}
	casbinkit.Reload(ctx, r.e, r.logger)
	return nil
}

func (r *Repo) AdminUserDeleteAtomic(ctx context.Context, id uint) error {
	if strconv.FormatUint(uint64(id), 10) == model.AdminUserID {
		return apiv1.ErrForbidden
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := casbinkit.NewTxEnforcer(tx)
		if err != nil {
			return err
		}
		if _, err := e.DeleteRolesForUser(strconv.FormatUint(uint64(id), 10)); err != nil {
			return err
		}
		return casbinkit.EnsureRowsAffected(tx, tx.Where("id = ?", id).Delete(&model.AdminUser{}), &model.AdminUser{}, id)
	}); err != nil {
		return err
	}
	casbinkit.Reload(ctx, r.e, r.logger)
	return nil
}
