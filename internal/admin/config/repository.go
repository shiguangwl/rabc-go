// Package config 管理运行时系统配置的增删改查。
//
// 设计取舍：配置不参与 Casbin 资源鉴权，Repo 为纯 DB 访问，无 Atomic 方法。
// 删除一律物理删除（Unscoped）：软删除残留行仍占用 config_key 唯一索引，
// 会导致删除后无法重建同名配置。
package config

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
)

// listOrder 固定列表排序，避免前端分组展示抖动。
const listOrder = "config_group ASC, weight DESC, id ASC"

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) ListAll(ctx context.Context) ([]model.SysConfig, error) {
	var list []model.SysConfig
	if err := r.db.WithContext(ctx).Order(listOrder).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repo) ListPublic(ctx context.Context) ([]model.SysConfig, error) {
	var list []model.SysConfig
	if err := r.db.WithContext(ctx).Where("is_public = ?", true).Order(listOrder).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repo) GetByID(ctx context.Context, id uint) (*model.SysConfig, error) {
	var cfg model.SysConfig
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %w", apiv1.ErrConfigKeyNotFound, err)
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *Repo) GetByKey(ctx context.Context, key string) (*model.SysConfig, error) {
	var cfg model.SysConfig
	if err := r.db.WithContext(ctx).Where("config_key = ?", key).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %w", apiv1.ErrConfigKeyNotFound, err)
		}
		return nil, err
	}
	return &cfg, nil
}

// FindByKeys 按 key 批量查询，用于批量更新前的校验快照。
func (r *Repo) FindByKeys(ctx context.Context, keys []string) ([]model.SysConfig, error) {
	var list []model.SysConfig
	if err := r.db.WithContext(ctx).Where("config_key IN ?", keys).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repo) Create(ctx context.Context, cfg *model.SysConfig) error {
	if err := r.db.WithContext(ctx).Create(cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("%w: %w", apiv1.ErrConfigKeyExists, err)
		}
		return err
	}
	return nil
}

// BatchUpdateValues 在单事务内逐 key 改值，全成功才提交。
//
// 每 key 校验 RowsAffected：若某 key 在校验快照之后被并发删除，影响行数为 0，
// 整批回滚为 ErrConfigKeyNotFound，杜绝部分成功。
func (r *Repo) BatchUpdateValues(ctx context.Context, kv map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range kv {
			res := tx.Model(&model.SysConfig{}).
				Where("config_key = ?", key).
				Update("config_value", value)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("%w: %s", apiv1.ErrConfigKeyNotFound, key)
			}
		}
		return nil
	})
}

func (r *Repo) DeleteByID(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.SysConfig{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apiv1.ErrConfigKeyNotFound
	}
	return nil
}
