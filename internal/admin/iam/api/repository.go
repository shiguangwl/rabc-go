package api

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

type Repo struct {
	db     *gorm.DB
	e      *casbin.SyncedEnforcer
	logger *log.Logger
	mu     *casbinkit.RBACMu
}

func NewRepo(db *gorm.DB, e *casbin.SyncedEnforcer, logger *log.Logger, mu *casbinkit.RBACMu) *Repo {
	return &Repo{db: db, e: e, logger: logger, mu: mu}
}

func (r *Repo) GetAPIGroups(ctx context.Context) ([]string, error) {
	res := make([]string, 0)
	if err := r.db.WithContext(ctx).Model(&model.API{}).Distinct().Order("group_name ASC").Pluck("group_name", &res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (r *Repo) GetApis(ctx context.Context, q Query) ([]model.API, int64, error) {
	var list []model.API
	var total int64
	scope := r.db.WithContext(ctx).Model(&model.API{})
	if q.Name != "" {
		scope = scope.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Group != "" {
		scope = scope.Where("group_name LIKE ?", "%"+q.Group+"%")
	}
	if q.Path != "" {
		scope = scope.Where("path LIKE ?", "%"+q.Path+"%")
	}
	if q.Method != "" {
		scope = scope.Where("method = ?", q.Method)
	}
	if err := scope.Count(&total).Error; err != nil {
		return nil, total, err
	}
	if err := scope.Offset(q.Offset()).Limit(q.Limit()).Order("group_name ASC").Find(&list).Error; err != nil {
		return nil, total, err
	}
	return list, total, nil
}

func (r *Repo) APICreate(ctx context.Context, m *model.API) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("%w: %w", apiv1.ErrConflict, err)
		}
		return err
	}
	return nil
}

// APIUpdateAtomic：path/method 变更时必须在同事务内按旧 (path, method) 清 Casbin 策略，
// 防止旧资源 key 残留授权；读旧值放在事务内，避免并发更新清错对象。
func (r *Repo) APIUpdateAtomic(ctx context.Context, m *model.API) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	updates := map[string]any{
		"group_name": m.Group,
		"name":       m.Name,
		"path":       m.Path,
		"method":     m.Method,
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.API
		if err := tx.Where("id = ?", m.ID).First(&old).Error; err != nil {
			return err
		}
		pathChanged := old.Path != m.Path || old.Method != m.Method
		if err := casbinkit.EnsureRowsAffected(tx, tx.Model(&model.API{}).Where("id = ?", m.ID).Updates(updates), &model.API{}, m.ID); err != nil {
			return err
		}
		if !pathChanged {
			return nil
		}
		e, err := casbinkit.NewTxEnforcer(tx)
		if err != nil {
			return err
		}
		return casbinkit.RemoveByObjectAct(e, model.APIResourcePrefix+old.Path, old.Method)
	}); err != nil {
		return err
	}
	casbinkit.Reload(ctx, r.e, r.logger)
	return nil
}

func (r *Repo) APIDeleteAtomic(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.API
		if err := tx.Where("id = ?", id).First(&old).Error; err != nil {
			return err
		}
		e, err := casbinkit.NewTxEnforcer(tx)
		if err != nil {
			return err
		}
		// 撤权先：清 Casbin 策略后再删 DB 行；任一步失败整体回滚，避免残留脏权限。
		if err := casbinkit.RemoveByObjectAct(e, model.APIResourcePrefix+old.Path, old.Method); err != nil {
			return err
		}
		return casbinkit.EnsureRowsAffected(tx, tx.Unscoped().Where("id = ?", id).Delete(&model.API{}), &model.API{}, id)
	}); err != nil {
		return err
	}
	casbinkit.Reload(ctx, r.e, r.logger)
	return nil
}
