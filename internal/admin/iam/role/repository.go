package role

import (
	"context"
	"errors"
	"fmt"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/admin/iam/casbinkit"
	"rabc-go/internal/model"
	"rabc-go/pkg/log"
)

// Repo 中带 Atomic 后缀的方法承诺：DB 写与 Casbin 策略变更在同一事务内完成，
// 任一步失败整体回滚。调用方不得在事务外自行拼装。
type Repo struct {
	db     *gorm.DB
	e      *casbin.SyncedEnforcer
	logger *log.Logger
	mu     *casbinkit.RBACMu
}

func NewRepo(db *gorm.DB, e *casbin.SyncedEnforcer, logger *log.Logger, mu *casbinkit.RBACMu) *Repo {
	return &Repo{db: db, e: e, logger: logger, mu: mu}
}

func (r *Repo) RoleUpdate(ctx context.Context, m *model.Role) error {
	result := r.db.WithContext(ctx).Model(&model.Role{}).Where("id = ?", m.ID).UpdateColumn("name", m.Name)
	return casbinkit.EnsureRowsAffected(r.db.WithContext(ctx), result, &model.Role{}, m.ID)
}

// RoleCreateIfAbsent 把 GORM 的通用 ErrDuplicatedKey 拆回 name/sid 两种业务错误，
// 让上层能区分"名字撞了"和"sid 撞了"。反查时若冲突行恰好消失（被并发删除），
// 按 sid 冲突兜底，避免吞错。
func (r *Repo) RoleCreateIfAbsent(ctx context.Context, m *model.Role) error {
	err := r.db.WithContext(ctx).Create(m).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return err
	}
	var existing model.Role
	if e := r.db.WithContext(ctx).Unscoped().Where("name = ?", m.Name).Limit(1).First(&existing).Error; e == nil {
		return fmt.Errorf("%w: %w", apiv1.ErrRoleNameExists, err)
	}
	if e := r.db.WithContext(ctx).Unscoped().Where("sid = ?", m.Sid).Limit(1).First(&existing).Error; e == nil {
		return fmt.Errorf("%w: %w", apiv1.ErrRoleSidExists, err)
	}
	return fmt.Errorf("%w: %w", apiv1.ErrRoleSidExists, err)
}

func (r *Repo) GetRoles(ctx context.Context, q Query) ([]model.Role, int64, error) {
	var list []model.Role
	var total int64
	scope := r.db.WithContext(ctx).Model(&model.Role{})
	if q.Name != "" {
		scope = scope.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Sid != "" {
		scope = scope.Where("sid = ?", q.Sid)
	}
	if err := scope.Count(&total).Error; err != nil {
		return nil, total, err
	}
	if err := scope.Offset(q.Offset()).Limit(q.Limit()).Find(&list).Error; err != nil {
		return nil, total, err
	}
	return list, total, nil
}

func (r *Repo) GetRole(ctx context.Context, id uint) (model.Role, error) {
	m := model.Role{}
	return m, r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
}

func (r *Repo) RoleDeleteAtomic(ctx context.Context, id uint, sid string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := casbinkit.NewTxEnforcer(tx)
		if err != nil {
			return err
		}
		// 撤权先：先清 Casbin 策略再删 DB 行；顺序反过来会出现"DB 已无角色但策略仍生效"的脏权限窗口。
		if _, err := e.DeleteRole(model.RoleSubject(sid)); err != nil {
			return err
		}
		return casbinkit.EnsureRowsAffected(tx, tx.Where("id = ?", id).Delete(&model.Role{}), &model.Role{}, id)
	}); err != nil {
		return err
	}
	casbinkit.Reload(ctx, r.e, r.logger)
	return nil
}
