package api

import (
	"context"

	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
)

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

type Service struct {
	repo *Repo
}

func (s *Service) GetApis(ctx context.Context, req *apiv1.GetApisRequest) (*apiv1.GetApisResponseData, error) {
	q := Query{
		All:    req.All,
		Group:  req.Group,
		Name:   req.Name,
		Path:   req.Path,
		Method: req.Method,
	}
	if !req.All {
		req.Normalize()
		q.Pagination = req.Pagination
	}
	list, total, err := s.repo.GetApis(ctx, q)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.GetAPIGroups(ctx)
	if err != nil {
		return nil, err
	}
	data := &apiv1.GetApisResponseData{
		List:   make([]apiv1.APIDataItem, 0),
		Total:  total,
		Groups: groups,
	}
	for _, m := range list {
		data.List = append(data.List, apiv1.APIDataItem{
			CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
			Group:     m.Group,
			ID:        m.ID,
			Method:    m.Method,
			Name:      m.Name,
			Path:      m.Path,
			UpdatedAt: m.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return data, nil
}

func (s *Service) APIUpdate(ctx context.Context, req *apiv1.APIUpdateRequest) error {
	return s.repo.APIUpdateAtomic(ctx, &model.API{
		Group:  req.Group,
		Method: req.Method,
		Name:   req.Name,
		Path:   req.Path,
		Model:  gorm.Model{ID: req.ID},
	})
}

// APICreate 走 repo 单写：新登记的 API 资源默认无 Casbin 策略绑定，需在角色权限管理中显式分配。
func (s *Service) APICreate(ctx context.Context, req *apiv1.APICreateRequest) error {
	return s.repo.APICreate(ctx, &model.API{
		Group:  req.Group,
		Method: req.Method,
		Name:   req.Name,
		Path:   req.Path,
	})
}

func (s *Service) APIDelete(ctx context.Context, id uint) error {
	return s.repo.APIDeleteAtomic(ctx, id)
}
