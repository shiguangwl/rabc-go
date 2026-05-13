package service

import (
	"context"

	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
	"rabc-go/internal/repository"
)

func (s *adminService) GetApis(ctx context.Context, req *apiv1.GetApisRequest) (*apiv1.GetApisResponseData, error) {
	req.Normalize()
	list, total, err := s.adminRepository.GetApis(ctx, repository.APIQuery{
		PageQuery: pageQuery(req.Pagination),
		Group:     req.Group,
		Name:      req.Name,
		Path:      req.Path,
		Method:    req.Method,
	})
	if err != nil {
		return nil, repositoryError(err)
	}
	groups, err := s.adminRepository.GetAPIGroups(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	data := &apiv1.GetApisResponseData{
		List:   make([]apiv1.APIDataItem, 0),
		Total:  total,
		Groups: groups,
	}
	for _, api := range list {
		data.List = append(data.List, apiv1.APIDataItem{
			CreatedAt: api.CreatedAt.Format("2006-01-02 15:04:05"),
			Group:     api.Group,
			ID:        api.ID,
			Method:    api.Method,
			Name:      api.Name,
			Path:      api.Path,
			UpdatedAt: api.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return data, nil
}

func (s *adminService) APIUpdate(ctx context.Context, req *apiv1.APIUpdateRequest) error {
	// path/method 变更时 repo 在同一事务内读取旧值并清旧 Casbin 策略；
	// 避免 service 先读旧值再更新导致并发修改时清错资源 key。
	return repositoryError(s.adminRepository.APIUpdateAtomic(ctx, &model.API{
		Group:  req.Group,
		Method: req.Method,
		Name:   req.Name,
		Path:   req.Path,
		Model:  gorm.Model{ID: req.ID},
	}))
}

// APICreate 不需要事务：新登记的 API 资源默认无任何 Casbin 策略绑定，
// 需要管理员通过"角色权限管理"页面显式分配。直接走 repo 单写即可。
func (s *adminService) APICreate(ctx context.Context, req *apiv1.APICreateRequest) error {
	return repositoryError(s.adminRepository.APICreate(ctx, &model.API{
		Group:  req.Group,
		Method: req.Method,
		Name:   req.Name,
		Path:   req.Path,
	}))
}

func (s *adminService) APIDelete(ctx context.Context, id uint) error {
	return repositoryError(s.adminRepository.APIDeleteAtomic(ctx, id))
}
