package menu

import (
	"context"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
	"rabc-go/pkg/log"
)

// PermissionReader 在消费侧定义（菜单按权限过滤需要读策略），由 permission 子域实现，
// 防止 menu → permission 反向依赖。
type PermissionReader interface {
	GetUserPermissions(ctx context.Context, uid uint) ([][]string, error)
}

func NewService(logger *log.Logger, repo *Repo, perms PermissionReader) *Service {
	return &Service{logger: logger, repo: repo, perms: perms}
}

type Service struct {
	logger *log.Logger
	repo   *Repo
	perms  PermissionReader
}

func (s *Service) MenuUpdate(ctx context.Context, req *apiv1.MenuUpdateRequest) error {
	return s.repo.MenuUpdateAtomic(ctx, &model.Menu{
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
		Model:      gorm.Model{ID: req.ID},
	})
}

func (s *Service) MenuCreate(ctx context.Context, req *apiv1.MenuCreateRequest) error {
	return s.repo.MenuCreate(ctx, &model.Menu{
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
	})
}

func (s *Service) MenuDelete(ctx context.Context, id uint) error {
	return s.repo.MenuDeleteAtomic(ctx, id)
}

func (s *Service) GetMenus(ctx context.Context, uid uint) (*apiv1.GetMenuResponseData, error) {
	menuList, err := s.repo.GetMenuList(ctx)
	if err != nil {
		s.logger.WithContext(ctx).Error("获取菜单列表失败", zap.Error(err))
		return nil, err
	}
	data := &apiv1.GetMenuResponseData{List: make([]apiv1.MenuDataItem, 0)}
	isAdmin := strconv.FormatUint(uint64(uid), 10) == model.AdminUserID
	if isAdmin {
		for _, m := range menuList {
			data.List = append(data.List, toItem(m))
		}
		return data, nil
	}

	permissions, err := s.perms.GetUserPermissions(ctx, uid)
	if err != nil {
		return nil, err
	}
	menuPermSet := map[string]struct{}{}
	for _, perm := range permissions {
		if len(perm) != 3 {
			continue
		}
		obj := perm[1]
		if !strings.HasPrefix(obj, model.MenuResourcePrefix) || perm[2] != "read" {
			continue
		}
		menuPermSet[strings.TrimPrefix(obj, model.MenuResourcePrefix)] = struct{}{}
	}

	for _, m := range menuList {
		if _, ok := menuPermSet[m.Path]; ok {
			data.List = append(data.List, toItem(m))
		}
	}
	return data, nil
}

func (s *Service) GetAdminMenus(ctx context.Context) (*apiv1.GetMenuResponseData, error) {
	menuList, err := s.repo.GetMenuList(ctx)
	if err != nil {
		s.logger.WithContext(ctx).Error("获取菜单列表失败", zap.Error(err))
		return nil, err
	}
	data := &apiv1.GetMenuResponseData{List: make([]apiv1.MenuDataItem, 0)}
	for _, m := range menuList {
		data.List = append(data.List, toItem(m))
	}
	return data, nil
}

func toItem(m model.Menu) apiv1.MenuDataItem {
	return apiv1.MenuDataItem{
		ID:         m.ID,
		Name:       m.Name,
		Title:      m.Title,
		Path:       m.Path,
		Component:  m.Component,
		Redirect:   m.Redirect,
		KeepAlive:  m.KeepAlive,
		HideInMenu: m.HideInMenu,
		Locale:     m.Locale,
		Weight:     m.Weight,
		Icon:       m.Icon,
		ParentID:   m.ParentID,
		UpdatedAt:  m.UpdatedAt.Format("2006-01-02 15:04:05"),
		URL:        m.URL,
	}
}
