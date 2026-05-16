package user

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
	"rabc-go/pkg/log"
)

// AuthRevoker 在消费侧定义，由 auth 子域实现，防止 user 反向依赖 auth。
// 契约：改密、删除等敏感写操作完成后必须调用，立即吊销该用户全部活跃 session，
// 否则旧 token 仍可继续访问，形成安全漏洞。
type AuthRevoker interface {
	RevokeAllUserSessions(ctx context.Context, uid uint, reason string) (int, error)
}

func NewService(logger *log.Logger, repo *Repo, revoker AuthRevoker) *Service {
	return &Service{logger: logger, repo: repo, revoker: revoker}
}

type Service struct {
	logger  *log.Logger
	repo    *Repo
	revoker AuthRevoker
}

func (s *Service) GetAdminUser(ctx context.Context, uid uint) (*apiv1.GetAdminUserResponseData, error) {
	user, err := s.repo.GetAdminUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	// 角色查询失败降级为空列表：详情页主信息仍可见，避免 Casbin 抖动连带阻塞用户面板。
	// 必须 warn 而非静默忽略，否则 Casbin 故障无可观测信号。
	roles, err := s.repo.GetUserRoles(ctx, uid)
	if err != nil {
		s.logger.WithContext(ctx).Warn("获取用户角色失败", zap.Uint("user_id", uid), zap.Error(err))
		roles = []string{}
	}

	return &apiv1.GetAdminUserResponseData{
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

// formatNullableTime 把 nil 视作"从未登录"，返回空串而非 "0001-01-01..."，
// 前端依此判断是否首次登录。
func formatNullableTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func (s *Service) GetAdminUsers(ctx context.Context, req *apiv1.GetAdminUsersRequest) (*apiv1.GetAdminUsersResponseData, error) {
	req.Normalize()
	list, total, err := s.repo.GetAdminUsers(ctx, Query{
		Pagination: req.Pagination,
		Username:   req.Username,
		Nickname:   req.Nickname,
		Email:      req.Email,
		Phone:      req.Phone,
	})
	if err != nil {
		return nil, err
	}
	data := &apiv1.GetAdminUsersResponseData{
		List:  make([]apiv1.AdminUserDataItem, 0),
		Total: total,
	}
	for _, user := range list {
		// 列表页与详情页相反：任一行角色查询失败立即整体失败，保证返回 List 长度与 Total 语义一致，
		// 否则前端分页会把"少了的行"误判为"被删了"。
		roles, err := s.repo.GetUserRoles(ctx, user.ID)
		if err != nil {
			s.logger.WithContext(ctx).Error("获取用户角色失败", zap.Uint("user_id", user.ID), zap.Error(err))
			return nil, err
		}
		data.List = append(data.List, apiv1.AdminUserDataItem{
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

func (s *Service) AdminUserUpdate(ctx context.Context, req *apiv1.AdminUserUpdateRequest) error {
	// 密码空 = 不改密码；当前请求模型无法表达"显式清空密码"，也不应支持（账号必须有密码）。
	passwordHash := ""
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		passwordHash = string(hash)
	}
	if err := s.repo.AdminUserUpdateAtomic(ctx, &model.AdminUser{
		Model:    gorm.Model{ID: req.ID},
		Email:    req.Email,
		Nickname: req.Nickname,
		Password: passwordHash,
		Phone:    req.Phone,
		Username: req.Username,
	}, req.Roles); err != nil {
		return err
	}

	// 安全契约：密码已成功落库后必须吊销全部活跃 session，否则旧 token 仍能继续访问。
	// 吊销失败仅 warn 不回滚——密码已变更是事实，回滚反而会留下"前端以为改了但其实没改"的更糟状态。
	if req.Password != "" {
		if _, err := s.revoker.RevokeAllUserSessions(ctx, req.ID, "password_change"); err != nil {
			s.logger.WithContext(ctx).Warn("改密后吊销会话失败",
				zap.Uint("user_id", req.ID), zap.Error(err))
		}
	}
	return nil
}

func (s *Service) AdminUserCreate(ctx context.Context, req *apiv1.AdminUserCreateRequest) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.AdminUserCreateAtomic(ctx, &model.AdminUser{
		Email:    req.Email,
		Nickname: req.Nickname,
		Phone:    req.Phone,
		Username: req.Username,
		Password: string(hash),
	}, req.Roles)
}

func (s *Service) AdminUserDelete(ctx context.Context, id uint) error {
	if err := s.repo.AdminUserDeleteAtomic(ctx, id); err != nil {
		return err
	}
	// 安全契约：删除用户后必须吊销活跃 session，否则被删账号的旧 token 仍能继续访问。
	// 吊销失败仅 warn 不回滚——删除已落库且不可逆，回滚 session 状态没有意义。
	if _, err := s.revoker.RevokeAllUserSessions(ctx, id, "delete"); err != nil {
		s.logger.WithContext(ctx).Warn("删除用户后吊销会话失败",
			zap.Uint("user_id", id), zap.Error(err))
	}
	return nil
}
