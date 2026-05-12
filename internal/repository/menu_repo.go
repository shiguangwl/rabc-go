package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"rabc-go/internal/model"
)

func (r *adminRepository) MenuCreate(ctx context.Context, m *model.Menu) error {
	if err := r.DB(ctx).Create(m).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("%w: %v", ErrConflict, err)
		}
		return err
	}
	return nil
}

func (r *adminRepository) GetMenuList(ctx context.Context) ([]model.Menu, error) {
	var menuList []model.Menu
	if err := r.DB(ctx).Order("weight DESC").Find(&menuList).Error; err != nil {
		return nil, err
	}
	return menuList, nil
}

func (r *adminRepository) MenuUpdateAtomic(ctx context.Context, m *model.Menu) error {
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
		var old model.Menu
		if err := tx.Where("id = ?", m.ID).First(&old).Error; err != nil {
			return err
		}
		pathChanged := old.Path != m.Path
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
		return removePoliciesByObjectActOn(e, model.MenuResourcePrefix+old.Path, "read")
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}

func (r *adminRepository) MenuDeleteAtomic(ctx context.Context, id uint) error {
	r.rbacMu.Lock()
	defer r.rbacMu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.Menu
		if err := tx.Where("id = ?", id).First(&old).Error; err != nil {
			return err
		}
		e, err := r.newTxEnforcer(tx)
		if err != nil {
			return err
		}
		if err := removePoliciesByObjectActOn(e, model.MenuResourcePrefix+old.Path, "read"); err != nil {
			return err
		}
		return ensureRowsAffected(tx, tx.Unscoped().Where("id = ?", id).Delete(&model.Menu{}), &model.Menu{}, id)
	}); err != nil {
		return err
	}
	r.reloadPolicy(ctx)
	return nil
}
