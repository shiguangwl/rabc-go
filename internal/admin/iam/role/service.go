package role

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// RoleUpdate 只允许改显示名 Name；Sid 是 Casbin 策略的关联键，变更会让所有授权失联。
func (s *Service) RoleUpdate(ctx context.Context, req *apiv1.RoleUpdateRequest) error {
	return s.repo.RoleUpdate(ctx, &model.Role{
		Name:  req.Name,
		Model: gorm.Model{ID: req.ID},
	})
}

func (s *Service) RoleCreate(ctx context.Context, req *apiv1.RoleCreateRequest) error {
	// 安全约束：禁止 sid 以 RoleSubjectPrefix 开头。Casbin role subject 形如 "role:<sid>"，
	// 若放行 sid="role:foo"，它经 RoleSubject 后会与正常 sid="foo" 撞名，破坏命名空间隔离，
	// 进而允许伪造他人角色的策略命中。入口拦截是唯一可靠的堵漏点。
	if strings.HasPrefix(req.Sid, model.RoleSubjectPrefix) {
		return apiv1.ErrBadRequest
	}
	return s.repo.RoleCreateIfAbsent(ctx, &model.Role{Name: req.Name, Sid: req.Sid})
}

func (s *Service) RoleDelete(ctx context.Context, id uint) error {
	old, err := s.repo.GetRole(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.RoleDeleteAtomic(ctx, id, old.Sid)
}

func (s *Service) GetRoles(ctx context.Context, req *apiv1.GetRoleListRequest) (*apiv1.GetRolesResponseData, error) {
	req.Normalize()
	list, total, err := s.repo.GetRoles(ctx, Query{
		Pagination: req.Pagination,
		Sid:        req.Sid,
		Name:       req.Name,
	})
	if err != nil {
		return nil, err
	}
	data := &apiv1.GetRolesResponseData{
		List:  make([]apiv1.RoleDataItem, 0),
		Total: total,
	}
	for _, role := range list {
		data.List = append(data.List, apiv1.RoleDataItem{
			ID:        role.ID,
			Name:      role.Name,
			Sid:       role.Sid,
			UpdatedAt: role.UpdatedAt.Format("2006-01-02 15:04:05"),
			CreatedAt: role.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return data, nil
}
