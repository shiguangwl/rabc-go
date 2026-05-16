package permission

import (
	"context"
	"net/http"
	"strings"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
)

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

type Service struct {
	repo *Repo
}

func (s *Service) GetUserPermissions(ctx context.Context, uid uint) (*apiv1.GetUserPermissionsData, error) {
	data := &apiv1.GetUserPermissionsData{List: []string{}}
	list, err := s.repo.GetUserPermissions(ctx, uid)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if len(v) == 3 {
			data.List = append(data.List, strings.Join([]string{v[1], v[2]}, model.PermSep))
		}
	}
	return data, nil
}

func (s *Service) GetRolePermissions(ctx context.Context, role string) (*apiv1.GetRolePermissionsData, error) {
	data := &apiv1.GetRolePermissionsData{List: []string{}}
	list, err := s.repo.GetRolePermissions(ctx, role)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if len(v) == 3 {
			data.List = append(data.List, strings.Join([]string{v[1], v[2]}, model.PermSep))
		}
	}
	return data, nil
}

func (s *Service) UpdateRolePermission(ctx context.Context, req *apiv1.UpdateRolePermissionRequest) error {
	permissions := map[string]struct{}{}
	for _, v := range req.List {
		if !isValidPermission(v) {
			return apiv1.ErrBadRequest
		}
		permissions[v] = struct{}{}
	}
	return s.repo.UpdateRolePermission(ctx, req.Role, permissions)
}

// isValidPermission 约束权限串格式：API 资源的 action 必须是 HTTP method；
// Menu 资源的 action 仅允许 "read"。
//
// 这个白名单是契约——前端写入与 EnsurePermissionResources 校验都依赖它，
// 放宽前需同步两侧。
func isValidPermission(raw string) bool {
	parts := strings.Split(raw, model.PermSep)
	if len(parts) != 2 {
		return false
	}
	resource, action := parts[0], parts[1]
	if resource == "" || action == "" {
		return false
	}
	switch {
	case strings.HasPrefix(resource, model.APIResourcePrefix):
		switch action {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead:
			return true
		default:
			return false
		}
	case strings.HasPrefix(resource, model.MenuResourcePrefix):
		return action == "read"
	default:
		return false
	}
}
