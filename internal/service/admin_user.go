package service

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	v1 "rabc-go/api/v1"
	"rabc-go/internal/model"
	"rabc-go/internal/repository"
)

func (s *adminService) GetAdminUser(ctx context.Context, uid uint) (*v1.GetAdminUserResponseData, error) {
	user, err := s.adminRepository.GetAdminUser(ctx, uid)
	if err != nil {
		return nil, repositoryError(err)
	}
	// 详情页查不到角色不阻塞主信息返回，但记录 warn 让 SRE 能感知到 Casbin 异常；
	// 静默 _ 忽略会让前端永远不知道角色查询挂掉。
	roles, err := s.adminRepository.GetUserRoles(ctx, uid)
	if err != nil {
		s.logger.WithContext(ctx).Warn("获取用户角色失败", zap.Uint("user_id", uid), zap.Error(err))
		roles = []string{}
	}

	return &v1.GetAdminUserResponseData{
		Email:       user.Email,
		ID:          user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Phone:       user.Phone,
		Roles:       roles,
		CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
		LastLoginAt: formatNullableTime(user.LastLoginAt),
	}, nil
}

// formatNullableTime 把 *time.Time 转字符串：nil → 空串，表示"从未登录"。
func formatNullableTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func (s *adminService) GetAdminUsers(ctx context.Context, req *v1.GetAdminUsersRequest) (*v1.GetAdminUsersResponseData, error) {
	req.Normalize()
	list, total, err := s.adminRepository.GetAdminUsers(ctx, repository.AdminUserQuery{
		PageQuery: pageQuery(req.Pagination),
		Username:  req.Username,
		Nickname:  req.Nickname,
		Email:     req.Email,
		Phone:     req.Phone,
	})
	if err != nil {
		return nil, repositoryError(err)
	}
	data := &v1.GetAdminUsersResponseData{
		List:  make([]v1.AdminUserDataItem, 0),
		Total: total,
	}
	for _, user := range list {
		// 角色查询失败时整体返回错误，保证 List 长度与 Total 语义一致。
		roles, err := s.adminRepository.GetUserRoles(ctx, user.ID)
		if err != nil {
			s.logger.WithContext(ctx).Error("获取用户角色失败", zap.Uint("user_id", user.ID), zap.Error(err))
			return nil, repositoryError(err)
		}
		data.List = append(data.List, v1.AdminUserDataItem{
			Email:       user.Email,
			ID:          user.ID,
			Nickname:    user.Nickname,
			Username:    user.Username,
			Phone:       user.Phone,
			Roles:       roles,
			CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   user.UpdatedAt.Format("2006-01-02 15:04:05"),
			LastLoginAt: formatNullableTime(user.LastLoginAt),
		})
	}
	return data, nil
}

func (s *adminService) AdminUserUpdate(ctx context.Context, req *v1.AdminUserUpdateRequest) error {
	// 密码为空表示不修改密码列；当前请求模型不支持显式清空密码。
	passwordHash := ""
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		passwordHash = string(hash)
	}
	if err := s.adminRepository.AdminUserUpdateAtomic(ctx, &model.AdminUser{
		Model:    gorm.Model{ID: req.ID},
		Email:    req.Email,
		Nickname: req.Nickname,
		Password: passwordHash,
		Phone:    req.Phone,
		Username: req.Username,
	}, req.Roles); err != nil {
		return repositoryError(err)
	}

	// 密码变更后吊销活跃会话；吊销失败不回滚已提交的账号信息。
	if req.Password != "" {
		if _, err := s.authService.RevokeAllUserSessions(ctx, req.ID, "password_change"); err != nil {
			s.logger.WithContext(ctx).Warn("改密后吊销会话失败",
				zap.Uint("user_id", req.ID), zap.Error(err))
		}
	}
	return nil
}

func (s *adminService) AdminUserCreate(ctx context.Context, req *v1.AdminUserCreateRequest) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return repositoryError(s.adminRepository.AdminUserCreateAtomic(ctx, &model.AdminUser{
		Email:    req.Email,
		Nickname: req.Nickname,
		Phone:    req.Phone,
		Username: req.Username,
		Password: string(hash),
	}, req.Roles))
}

func (s *adminService) AdminUserDelete(ctx context.Context, id uint) error {
	if err := s.adminRepository.AdminUserDeleteAtomic(ctx, id); err != nil {
		return repositoryError(err)
	}
	// 删除用户后吊销活跃会话；吊销失败不回滚已提交的删除操作。
	if _, err := s.authService.RevokeAllUserSessions(ctx, id, "delete"); err != nil {
		s.logger.WithContext(ctx).Warn("删除用户后吊销会话失败",
			zap.Uint("user_id", id), zap.Error(err))
	}
	return nil
}
