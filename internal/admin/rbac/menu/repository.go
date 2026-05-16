// Package menu 管理菜单元数据，并按用户权限过滤菜单可见性。
//
// 不变量：菜单以 path 为权限身份；path 变更或菜单删除必须同事务清 (menu:<old_path>, read)
// 的 Casbin 策略，否则会出现"菜单已删，授权仍在"的脏权限。
package menu

import (
	"context"
	"errors"
	"fmt"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/admin/rbac/casbinkit"
	"rabc-go/internal/model"
	"rabc-go/pkg/log"
)

// Repo 中带 Atomic 后缀的方法承诺：DB 写与 Casbin 策略清理在同一事务内完成，
// 任一步失败整体回滚。调用方不得在事务外自行拼装这两步。
type Repo struct {
	db     *gorm.DB
	e      *casbin.SyncedEnforcer
	logger *log.Logger
	mu     *casbinkit.RBACMu
}

func NewRepo(db *gorm.DB, e *casbin.SyncedEnforcer, logger *log.Logger, mu *casbinkit.RBACMu) *Repo {
	return &Repo{db: db, e: e, logger: logger, mu: mu}
}

func (r *Repo) MenuCreate(ctx context.Context, m *model.Menu) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("%w: %w", apiv1.ErrConflict, err)
		}
		return err
	}
	return nil
}

func (r *Repo) GetMenuList(ctx context.Context) ([]model.Menu, error) {
	var menuList []model.Menu
	if err := r.db.WithContext(ctx).Order("weight DESC").Find(&menuList).Error; err != nil {
		return nil, err
	}
	return menuList, nil
}

func (r *Repo) MenuUpdateAtomic(ctx context.Context, m *model.Menu) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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
		if err := casbinkit.EnsureRowsAffected(tx, tx.Model(&model.Menu{}).Where("id = ?", m.ID).Updates(updates), &model.Menu{}, m.ID); err != nil {
			return err
		}
		if !pathChanged {
			return nil
		}
		e, err := casbinkit.NewTxEnforcer(tx)
		if err != nil {
			return err
		}
		return casbinkit.RemoveByObjectAct(e, model.MenuResourcePrefix+old.Path, "read")
	}); err != nil {
		return err
	}
	casbinkit.Reload(ctx, r.e, r.logger)
	return nil
}

func (r *Repo) MenuDeleteAtomic(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.Menu
		if err := tx.Where("id = ?", id).First(&old).Error; err != nil {
			return err
		}
		e, err := casbinkit.NewTxEnforcer(tx)
		if err != nil {
			return err
		}
		if err := casbinkit.RemoveByObjectAct(e, model.MenuResourcePrefix+old.Path, "read"); err != nil {
			return err
		}
		return casbinkit.EnsureRowsAffected(tx, tx.Unscoped().Where("id = ?", id).Delete(&model.Menu{}), &model.Menu{}, id)
	}); err != nil {
		return err
	}
	casbinkit.Reload(ctx, r.e, r.logger)
	return nil
}
