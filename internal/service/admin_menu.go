package service

import (
	"context"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	v1 "rabc-go/api/v1"
	"rabc-go/internal/model"
)

func (s *adminService) MenuUpdate(ctx context.Context, req *v1.MenuUpdateRequest) error {
	return repositoryError(s.adminRepository.MenuUpdateAtomic(ctx, &model.Menu{
		Component:  req.Component,
		Icon:       req.Icon,
		KeepAlive:  req.KeepAlive,
		HideInMenu: req.HideInMenu,
		Locale:     req.Locale,
		Weight:     req.Weight,
		Name:       req.Name,
		ParentID:   req.ParentID,
		Path:       req.Path,
		Redirect:   req.Redirect,
		Title:      req.Title,
		URL:        req.URL,
		Model: gorm.Model{
			ID: req.ID,
		},
	}))
}

func (s *adminService) MenuCreate(ctx context.Context, req *v1.MenuCreateRequest) error {
	return repositoryError(s.adminRepository.MenuCreate(ctx, &model.Menu{
		Component:  req.Component,
		Icon:       req.Icon,
		KeepAlive:  req.KeepAlive,
		HideInMenu: req.HideInMenu,
		Locale:     req.Locale,
		Weight:     req.Weight,
		Name:       req.Name,
		ParentID:   req.ParentID,
		Path:       req.Path,
		Redirect:   req.Redirect,
		Title:      req.Title,
		URL:        req.URL,
	}))
}

func (s *adminService) MenuDelete(ctx context.Context, id uint) error {
	return repositoryError(s.adminRepository.MenuDeleteAtomic(ctx, id))
}

func (s *adminService) GetMenus(ctx context.Context, uid uint) (*v1.GetMenuResponseData, error) {
	menuList, err := s.adminRepository.GetMenuList(ctx)
	if err != nil {
		s.logger.WithContext(ctx).Error("获取菜单列表失败", zap.Error(err))
		return nil, repositoryError(err)
	}
	data := &v1.GetMenuResponseData{
		List: make([]v1.MenuDataItem, 0),
	}
	isAdmin := strconv.FormatUint(uint64(uid), 10) == model.AdminUserID
	if isAdmin {
		for _, menu := range menuList {
			data.List = append(data.List, menuDataItem(menu))
		}
		return data, nil
	}

	// 获取权限的菜单
	permissions, err := s.adminRepository.GetUserPermissions(ctx, uid)
	if err != nil {
		return nil, repositoryError(err)
	}
	menuPermMap := map[string]struct{}{}
	for _, permission := range permissions {
		if len(permission) != 3 {
			continue
		}
		obj := permission[1]
		if !strings.HasPrefix(obj, model.MenuResourcePrefix) || permission[2] != "read" {
			continue
		}
		menuPermMap[strings.TrimPrefix(obj, model.MenuResourcePrefix)] = struct{}{}
	}

	for _, menu := range menuList {
		if _, ok := menuPermMap[menu.Path]; ok {
			data.List = append(data.List, menuDataItem(menu))
		}
	}
	return data, nil
}
func (s *adminService) GetAdminMenus(ctx context.Context) (*v1.GetMenuResponseData, error) {
	menuList, err := s.adminRepository.GetMenuList(ctx)
	if err != nil {
		s.logger.WithContext(ctx).Error("获取菜单列表失败", zap.Error(err))
		return nil, repositoryError(err)
	}
	data := &v1.GetMenuResponseData{
		List: make([]v1.MenuDataItem, 0),
	}
	for _, menu := range menuList {
		data.List = append(data.List, menuDataItem(menu))
	}
	return data, nil
}

func menuDataItem(menu model.Menu) v1.MenuDataItem {
	return v1.MenuDataItem{
		ID:         menu.ID,
		Name:       menu.Name,
		Title:      menu.Title,
		Path:       menu.Path,
		Component:  menu.Component,
		Redirect:   menu.Redirect,
		KeepAlive:  menu.KeepAlive,
		HideInMenu: menu.HideInMenu,
		Locale:     menu.Locale,
		Weight:     menu.Weight,
		Icon:       menu.Icon,
		ParentID:   menu.ParentID,
		UpdatedAt:  menu.UpdatedAt.Format("2006-01-02 15:04:05"),
		URL:        menu.URL,
	}
}
