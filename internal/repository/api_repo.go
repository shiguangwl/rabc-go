package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"rabc-go/internal/model"
)

func (r *adminRepository) GetAPIGroups(ctx context.Context) ([]string, error) {
	res := make([]string, 0)
	if err := r.DB(ctx).Model(&model.API{}).Distinct().Order("group_name ASC").Pluck("group_name", &res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (r *adminRepository) GetApis(ctx context.Context, q APIQuery) ([]model.API, int64, error) {
	var list []model.API
	var total int64
	scope := r.DB(ctx).Model(&model.API{})
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

func (r *adminRepository) APICreate(ctx context.Context, m *model.API) error {
	if err := r.DB(ctx).Create(m).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("%w: %w", ErrConflict, err)
		}
		return err
	}
	return nil
}

func (r *adminRepository) APIUpdateAtomic(ctx context.Context, m *model.API) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
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
		if err := ensureRowsAffected(tx, tx.Model(&model.API{}).Where("id = ?", m.ID).Updates(updates), &model.API{}, m.ID); err != nil {
			return err
		}
		if !pathChanged {
			return nil
		}
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		return removePoliciesByObjectActOn(e, model.APIResourcePrefix+old.Path, old.Method)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) APIDeleteAtomic(ctx context.Context, id uint) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.API
		if err := tx.Where("id = ?", id).First(&old).Error; err != nil {
			return err
		}
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		// 撤权先：清 Casbin 策略后再删 DB 行
		if err := removePoliciesByObjectActOn(e, model.APIResourcePrefix+old.Path, old.Method); err != nil {
			return err
		}
		return ensureRowsAffected(tx, tx.Unscoped().Where("id = ?", id).Delete(&model.API{}), &model.API{}, id)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}
