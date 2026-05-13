package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"rabc-go/internal/model"
)

func (r *adminRepository) RoleUpdate(ctx context.Context, m *model.Role) error {
	result := r.DB(ctx).Model(&model.Role{}).Where("id = ?", m.ID).UpdateColumn("name", m.Name)
	return ensureRowsAffected(r.DB(ctx), result, &model.Role{}, m.ID)
}

// RoleCreateIfAbsent 在唯一约束冲突时保留 name/sid 的业务语义。
// 冲突行若在反查前消失，按 sid 冲突返回。
func (r *adminRepository) RoleCreateIfAbsent(ctx context.Context, m *model.Role) error {
	err := r.DB(ctx).Create(m).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return err
	}
	var existing model.Role
	if e := r.db.WithContext(ctx).Unscoped().Where("name = ?", m.Name).Limit(1).First(&existing).Error; e == nil {
		return fmt.Errorf("%w: %w", ErrRoleNameDuplicated, err)
	}
	if e := r.db.WithContext(ctx).Unscoped().Where("sid = ?", m.Sid).Limit(1).First(&existing).Error; e == nil {
		return fmt.Errorf("%w: %w", ErrRoleSIDDuplicated, err)
	}
	return fmt.Errorf("%w: %w", ErrRoleSIDDuplicated, err)
}

func (r *adminRepository) GetRoles(ctx context.Context, q RoleQuery) ([]model.Role, int64, error) {
	var list []model.Role
	var total int64
	scope := r.DB(ctx).Model(&model.Role{})
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

func (r *adminRepository) GetRole(ctx context.Context, id uint) (model.Role, error) {
	m := model.Role{}
	return m, r.DB(ctx).Where("id = ?", id).First(&m).Error
}
