// internal/service/auth.go 是 Auth 子系统的应用层，负责双 Token 颁发、轮换、
// 复用检测、主动登出与会话吊销。
//
// Handler 只处理 HTTP 绑定与响应，Redis key、RT hash 和轮换语义必须收敛在本层。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	v1 "rabc-go/api/v1"
	"rabc-go/internal/auth"
	"rabc-go/internal/repository"
)

// AuthService Auth 子系统应用层接口。
type AuthService interface {
	Login(ctx context.Context, req *v1.LoginRequest) (*LoginResult, error)
	Refresh(ctx context.Context, req *v1.RefreshRequest) (*RefreshResult, error)
	Logout(ctx context.Context, req *v1.LogoutRequest) error
	RevokeAllUserSessions(ctx context.Context, uid uint, reason string) (int, error)
	ListUserSessions(ctx context.Context, uid uint) ([]repository.SessionInfo, error)
	KickSession(ctx context.Context, uid uint, sid string) error
}

// LoginResult Login 返回结果，由 handler 转 LoginResponseData 给前端。
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // access TTL 秒数
}

// RefreshResult Refresh 返回结果。
type RefreshResult struct {
	AccessToken  string
	RefreshToken string // 新 RT（轮换语义，前端用新值覆盖旧值）
	ExpiresIn    int64
}

// NewAuthService 构造 AuthService。
//
// authService 只接收业务接口和 AuthConfig，避免应用层直接依赖 Redis 客户端。
func NewAuthService(
	service *Service,
	authRepo repository.AuthRepository,
	adminRepo repository.AdminRepository,
	authCfg *auth.AuthConfig,
) AuthService {
	return &authService{
		Service:   service,
		authRepo:  authRepo,
		adminRepo: adminRepo,
		cfg:       authCfg,
	}
}

type authService struct {
	*Service
	authRepo  repository.AuthRepository
	adminRepo repository.AdminRepository
	cfg       *auth.AuthConfig
}

// Login 登录路径。
//
// 登录失败路径必须避免暴露用户名是否存在；成功后必须同时写入 refresh 记录与
// sessions 索引，保证后续吊销能覆盖该会话。
func (s *authService) Login(ctx context.Context, req *v1.LoginRequest) (*LoginResult, error) {
	user, err := s.adminRepo.GetAdminUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 保持用户名不存在与密码错误的耗时接近，避免登录枚举侧信道。
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
			return nil, v1.ErrUnauthorized
		}
		return nil, v1.ErrInternalServerError.WithCause(err)
	}

	if user.IsDisabled {
		s.logger.WithContext(ctx).Warn("auth.login_disabled",
			zap.Uint("uid", user.ID), zap.String("username", user.Username))
		return nil, v1.ErrUserDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, v1.ErrUnauthorized
		}
		return nil, v1.ErrInternalServerError.WithCause(err)
	}

	rt, sid, err := repository.GenRT()
	if err != nil {
		return nil, v1.ErrInternalServerError.WithCause(err)
	}
	access, err := s.jwt.GenToken(user.ID, time.Now().Add(s.cfg.AccessTTL),
		map[string]any{"sid": sid})
	if err != nil {
		return nil, v1.ErrInternalServerError.WithCause(err)
	}

	exp := time.Now().Add(s.cfg.RefreshTTL).Unix()
	recordJSON, err := buildRefreshRecord(rt, user.ID, exp)
	if err != nil {
		return nil, v1.ErrInternalServerError.WithCause(err)
	}
	if err := s.authRepo.LoginCreate(ctx, repository.LoginCreateParams{
		UID:           user.ID,
		SID:           sid,
		RecordJSON:    recordJSON,
		RefreshTTLSec: int(s.cfg.RefreshTTL.Seconds()),
		ExpTS:         exp,
	}); err != nil {
		return nil, v1.ErrInternalServerError.WithCause(err)
	}

	// 最后登录时间仅用于审计展示，写入失败不影响本次登录。
	if err := s.adminRepo.UpdateLastLogin(ctx, user.ID, time.Now()); err != nil {
		s.logger.WithContext(ctx).Warn("auth.update_last_login_failed",
			zap.Uint("uid", user.ID), zap.Error(err))
	}

	s.logger.WithContext(ctx).Info("auth.login_success",
		zap.Uint("uid", user.ID), zap.String("sid", sid))

	return &LoginResult{
		AccessToken:  access,
		RefreshToken: rt,
		ExpiresIn:    int64(s.cfg.AccessTTL.Seconds()),
	}, nil
}

// Refresh 刷新路径。
//
// 刷新必须轮换 refresh token；检测到旧 RT 复用时必须吊销该用户全部会话。
func (s *authService) Refresh(ctx context.Context, req *v1.RefreshRequest) (*RefreshResult, error) {
	sid, _, err := repository.ParseRT(req.RefreshToken)
	if err != nil {
		// 对外不暴露 refresh token 的具体格式错误。
		return nil, v1.ErrUnauthorized
	}

	rec, err := s.authRepo.GetRefreshRecord(ctx, sid)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			// record 缺失时必须检查墓碑，区分自然过期和旧 RT 复用。
			tombUID, tombErr := s.authRepo.GetTombUID(ctx, sid)
			if tombErr == nil && tombUID > 0 {
				count, revokeErr := s.authRepo.RevokeAllUserSessions(ctx, tombUID)
				s.logger.WithContext(ctx).Warn("auth.reuse_detected",
					zap.Uint("uid", tombUID), zap.String("sid", sid),
					zap.String("via", "tomb"),
					zap.Int("revoked_count", count), zap.Error(revokeErr))
				return nil, v1.ErrRefreshReused
			}
			return nil, v1.ErrRefreshExpired
		}
		if errors.Is(err, repository.ErrRefreshRecordCorrupted) {
			tombUID, tombErr := s.authRepo.GetTombUID(ctx, sid)
			if tombErr == nil && tombUID > 0 {
				count, revokeErr := s.authRepo.RevokeAllUserSessions(ctx, tombUID)
				s.logger.WithContext(ctx).Warn("auth.reuse_detected",
					zap.Uint("uid", tombUID), zap.String("sid", sid),
					zap.String("via", "corrupted_record_tomb"),
					zap.Int("revoked_count", count), zap.Error(revokeErr))
				return nil, v1.ErrRefreshReused
			}
			s.logger.WithContext(ctx).Warn("auth.refresh_record_corrupted",
				zap.String("sid", sid), zap.Error(err))
			return nil, v1.ErrRefreshReused
		}
		return nil, v1.ErrInternalServerError.WithCause(err)
	}
	uid := rec.UID

	newRT, newSID, err := repository.GenRT()
	if err != nil {
		return nil, v1.ErrInternalServerError.WithCause(err)
	}
	newAccess, err := s.jwt.GenToken(uid, time.Now().Add(s.cfg.AccessTTL),
		map[string]any{"sid": newSID})
	if err != nil {
		return nil, v1.ErrInternalServerError.WithCause(err)
	}
	newExp := time.Now().Add(s.cfg.RefreshTTL).Unix()
	newRecord, err := buildRefreshRecord(newRT, uid, newExp)
	if err != nil {
		return nil, v1.ErrInternalServerError.WithCause(err)
	}

	err = s.authRepo.RotateRefresh(ctx, repository.RotateParams{
		UID:           uid,
		OldSID:        sid,
		NewSID:        newSID,
		ExpectedHash:  repository.HashRT(req.RefreshToken),
		NewRecordJSON: newRecord,
		RefreshTTLSec: int(s.cfg.RefreshTTL.Seconds()),
		TombTTLSec:    int(s.cfg.RotationTombTTL.Seconds()),
		NewExpTS:      newExp,
	})

	switch {
	case err == nil:
		s.logger.WithContext(ctx).Info("auth.refresh_success",
			zap.Uint("uid", uid), zap.String("old_sid", sid), zap.String("new_sid", newSID))
		return &RefreshResult{
			AccessToken:  newAccess,
			RefreshToken: newRT,
			ExpiresIn:    int64(s.cfg.AccessTTL.Seconds()),
		}, nil

	case errors.Is(err, repository.ErrRotateExpired):
		return nil, v1.ErrRefreshExpired

	case errors.Is(err, repository.ErrRotateReused):
		// hash mismatch 表示当前 sid 的 RT 被复用或篡改，必须吊销该用户全部会话。
		count, revokeErr := s.authRepo.RevokeAllUserSessions(ctx, uid)
		s.logger.WithContext(ctx).Warn("auth.reuse_detected",
			zap.Uint("uid", uid), zap.String("sid", sid),
			zap.String("via", "lua"),
			zap.Int("revoked_count", count), zap.Error(revokeErr))
		return nil, v1.ErrRefreshReused

	default:
		return nil, v1.ErrInternalServerError.WithCause(err)
	}
}

// Logout 登出路径：删除单个 session，不连坐其他 session。
//
// 不要求 access 有效——即便 access 过期用户也能完成登出。
func (s *authService) Logout(ctx context.Context, req *v1.LogoutRequest) error {
	sid, _, err := repository.ParseRT(req.RefreshToken)
	if err != nil {
		// 格式错也算成功登出（前端清 token 即可，不必报错）
		return nil
	}
	rec, err := s.authRepo.GetRefreshRecord(ctx, sid)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			// 已不存在，幂等成功
			return nil
		}
		return v1.ErrInternalServerError.WithCause(err)
	}
	if err := s.authRepo.DeleteSession(ctx, rec.UID, sid); err != nil &&
		!errors.Is(err, repository.ErrSessionNotFound) {
		return v1.ErrInternalServerError.WithCause(err)
	}
	s.logger.WithContext(ctx).Info("auth.logout",
		zap.Uint("uid", rec.UID), zap.String("sid", sid))
	return nil
}

// RevokeAllUserSessions 吊销指定用户的全部 session。
//
// reason 是审计字段：password_change / disable / delete / reuse_detected / admin_kick 等。
func (s *authService) RevokeAllUserSessions(ctx context.Context, uid uint, reason string) (int, error) {
	count, err := s.authRepo.RevokeAllUserSessions(ctx, uid)
	s.logger.WithContext(ctx).Warn("auth.revoke_all",
		zap.Uint("uid", uid),
		zap.String("reason", reason),
		zap.Int("sid_count", count),
		zap.Error(err))
	if err != nil {
		return count, v1.ErrInternalServerError.WithCause(err)
	}
	return count, nil
}

// ListUserSessions 列出该 user 活跃 session（管理端会话查看路径）。
func (s *authService) ListUserSessions(ctx context.Context, uid uint) ([]repository.SessionInfo, error) {
	return s.authRepo.ListUserSessions(ctx, uid)
}

// KickSession 踢下线单个 session（管理端踢下线路径）。
func (s *authService) KickSession(ctx context.Context, uid uint, sid string) error {
	if err := s.authRepo.DeleteSession(ctx, uid, sid); err != nil &&
		!errors.Is(err, repository.ErrSessionNotFound) {
		return v1.ErrInternalServerError.WithCause(err)
	}
	s.logger.WithContext(ctx).Warn("auth.admin_kick",
		zap.Uint("uid", uid), zap.String("sid", sid))
	return nil
}

// buildRefreshRecord 把 RT raw、uid、exp 序列化为 Redis 存储用的 JSON。
//
// 字段约定与 repository.RefreshRecord 对齐：
//
//	th  = sha256(rt_raw) hex（hash 校验用，不存原始 RT）
//	uid = 关联用户 ID（用于 RevokeAll 时定位）
//	exp = 失效时间戳（秒），同步写入 sessions ZSet 的 score
func buildRefreshRecord(rtRaw string, uid uint, exp int64) (string, error) {
	rec := repository.RefreshRecord{
		TokenHash: repository.HashRT(rtRaw),
		UID:       uid,
		Exp:       exp,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return "", fmt.Errorf("marshal refresh record: %w", err)
	}
	return string(b), nil
}
