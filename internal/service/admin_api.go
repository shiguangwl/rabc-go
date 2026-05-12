package service

import (
	"context"

	"gorm.io/gorm"

	v1 "rabc-go/api/v1"
	"rabc-go/internal/model"
	"rabc-go/internal/repository"
)

func (s *adminService) GetApis(ctx context.Context, req *v1.GetApisRequest) (*v1.GetApisResponseData, error) {
	req.Normalize()
	list, total, err := s.adminRepository.GetApis(ctx, repository.ApiQuery{
		PageQuery: pageQuery(req.Pagination),
		Group:     req.Group,
		Name:      req.Name,
		Path:      req.Path,
		Method:    req.Method,
	})
	if err != nil {
		return nil, repositoryError(err)
	}
	groups, err := s.adminRepository.GetApiGroups(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	data := &v1.GetApisResponseData{
		List:   make([]v1.ApiDataItem, 0),
		Total:  total,
		Groups: groups,
	}
	for _, api := range list {
		data.List = append(data.List, v1.ApiDataItem{
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

func (s *adminService) ApiUpdate(ctx context.Context, req *v1.ApiUpdateRequest) error {
	// path/method 变更时 repo 在同一事务内读取旧值并清旧 Casbin 策略；
	// 避免 service 先读旧值再更新导致并发修改时清错资源 key。
	return repositoryError(s.adminRepository.ApiUpdateAtomic(ctx, &model.Api{
		Group:  req.Group,
		Method: req.Method,
		Name:   req.Name,
		Path:   req.Path,
		Model:  gorm.Model{ID: req.ID},
	}))
}

// ApiCreate 不需要事务：新登记的 API 资源默认无任何 Casbin 策略绑定，
// 需要管理员通过"角色权限管理"页面显式分配。直接走 repo 单写即可。
func (s *adminService) ApiCreate(ctx context.Context, req *v1.ApiCreateRequest) error {
	return repositoryError(s.adminRepository.ApiCreate(ctx, &model.Api{
		Group:  req.Group,
		Method: req.Method,
		Name:   req.Name,
		Path:   req.Path,
	}))
}

func (s *adminService) ApiDelete(ctx context.Context, id uint) error {
	return repositoryError(s.adminRepository.ApiDeleteAtomic(ctx, id))
}
