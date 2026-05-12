package service

import (
	"context"
	"strings"

	"gorm.io/gorm"

	v1 "rabc-go/api/v1"
	"rabc-go/internal/model"
	"rabc-go/internal/repository"
)

// RoleUpdate 仅更新角色显示名 Name；Sid 是 Casbin 策略的关联键，不允许变更。
func (s *adminService) RoleUpdate(ctx context.Context, req *v1.RoleUpdateRequest) error {
	return repositoryError(s.adminRepository.RoleUpdate(ctx, &model.Role{
		Name: req.Name,
		Model: gorm.Model{
			ID: req.ID,
		},
	}))
}

func (s *adminService) RoleCreate(ctx context.Context, req *v1.RoleCreateRequest) error {
	// 禁止 sid 以 RoleSubjectPrefix 开头：Casbin 内部 role subject 形如 "role:<sid>"，
	// 若外部传入 sid="role:foo" 会与正常 sid="foo" 的 RoleSubject 结果撞名，
	// 让命名空间隔离失效。model.RoleSubject 出于幂等保留了短路逻辑，
	// 真正堵漏点在入口校验。
	if strings.HasPrefix(req.Sid, model.RoleSubjectPrefix) {
		return v1.ErrBadRequest
	}
	return repositoryError(s.adminRepository.RoleCreateIfAbsent(ctx, &model.Role{Name: req.Name, Sid: req.Sid}))
}

func (s *adminService) RoleDelete(ctx context.Context, id uint) error {
	old, err := s.adminRepository.GetRole(ctx, id)
	if err != nil {
		return repositoryError(err)
	}
	return repositoryError(s.adminRepository.RoleDeleteAtomic(ctx, id, old.Sid))
}

func (s *adminService) GetRoles(ctx context.Context, req *v1.GetRoleListRequest) (*v1.GetRolesResponseData, error) {
	req.Normalize()
	list, total, err := s.adminRepository.GetRoles(ctx, repository.RoleQuery{
		PageQuery: pageQuery(req.Pagination),
		Sid:       req.Sid,
		Name:      req.Name,
	})
	if err != nil {
		return nil, repositoryError(err)
	}
	data := &v1.GetRolesResponseData{
		List:  make([]v1.RoleDataItem, 0),
		Total: total,
	}
	for _, role := range list {
		data.List = append(data.List, v1.RoleDataItem{
			ID:        role.ID,
			Name:      role.Name,
			Sid:       role.Sid,
			UpdatedAt: role.UpdatedAt.Format("2006-01-02 15:04:05"),
			CreatedAt: role.CreatedAt.Format("2006-01-02 15:04:05"),
		})

	}
	return data, nil
}
