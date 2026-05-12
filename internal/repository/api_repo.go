package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"rabc-go/internal/model"
)

func (r *adminRepository) GetApiGroups(ctx context.Context) ([]string, error) {
	res := make([]string, 0)
	if err := r.DB(ctx).Model(&model.Api{}).Distinct().Order("group_name ASC").Pluck("group_name", &res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (r *adminRepository) GetApis(ctx context.Context, q ApiQuery) ([]model.Api, int64, error) {
	var list []model.Api
	var total int64
	scope := r.DB(ctx).Model(&model.Api{})
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

func (r *adminRepository) ApiCreate(ctx context.Context, m *model.Api) error {
	if err := r.DB(ctx).Create(m).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("%w: %v", ErrConflict, err)
		}
		return err
	}
	return nil
}

func (r *adminRepository) ApiUpdateAtomic(ctx context.Context, m *model.Api) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	updates := map[string]any{
		"group_name": m.Group,
		"name":       m.Name,
		"path":       m.Path,
		"method":     m.Method,
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.Api
		if err := tx.Where("id = ?", m.ID).First(&old).Error; err != nil {
			return err
		}
		pathChanged := old.Path != m.Path || old.Method != m.Method
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
		return removePoliciesByObjectActOn(e, model.ApiResourcePrefix+old.Path, old.Method)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) ApiDeleteAtomic(ctx context.Context, id uint) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.Api
		if err := tx.Where("id = ?", id).First(&old).Error; err != nil {
			return err
		}
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		// 撤权先：清 Casbin 策略后再删 DB 行
		if err := removePoliciesByObjectActOn(e, model.ApiResourcePrefix+old.Path, old.Method); err != nil {
			return err
		}
		return ensureRowsAffected(tx, tx.Unscoped().Where("id = ?", id).Delete(&model.Api{}), &model.Api{}, id)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}
