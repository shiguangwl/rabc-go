package service

import (
	"context"
	"net/http"
	"strings"

	v1 "rabc-go/api/v1"
	"rabc-go/internal/model"
)

func (s *adminService) GetUserPermissions(ctx context.Context, uid uint) (*v1.GetUserPermissionsData, error) {
	data := &v1.GetUserPermissionsData{List: []string{}}
	list, err := s.adminRepository.GetUserPermissions(ctx, uid)
	if err != nil {
		return nil, repositoryError(err)
	}
	for _, v := range list {
		if len(v) == 3 {
			data.List = append(data.List, strings.Join([]string{v[1], v[2]}, model.PermSep))
		}
	}
	return data, nil
}

func (s *adminService) GetRolePermissions(ctx context.Context, role string) (*v1.GetRolePermissionsData, error) {
	data := &v1.GetRolePermissionsData{List: []string{}}
	list, err := s.adminRepository.GetRolePermissions(ctx, role)
	if err != nil {
		return nil, repositoryError(err)
	}
	for _, v := range list {
		if len(v) == 3 {
			data.List = append(data.List, strings.Join([]string{v[1], v[2]}, model.PermSep))
		}
	}
	return data, nil
}

func (s *adminService) UpdateRolePermission(ctx context.Context, req *v1.UpdateRolePermissionRequest) error {
	permissions := map[string]struct{}{}
	for _, v := range req.List {
		if !isValidPermission(v) {
			return v1.ErrBadRequest
		}
		permissions[v] = struct{}{}
	}
	return repositoryError(s.adminRepository.UpdateRolePermission(ctx, req.Role, permissions))
}

// 校验权限串格式：API 权限必须携带 HTTP method，菜单权限只接受 read。
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
	case strings.HasPrefix(resource, model.ApiResourcePrefix):
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
