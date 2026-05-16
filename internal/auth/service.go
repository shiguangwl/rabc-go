package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/model"
	"rabc-go/pkg/jwt"
	"rabc-go/pkg/log"
)

// UserLookup 在本域定义、由 user 子域 Repo 实现：依赖倒置切断 auth → user
// 的反向 import，避免循环。新增方法须慎重，扩大此接口等于扩大 auth 对 user 的耦合面。
type UserLookup interface {
	GetAdminUserByUsername(ctx context.Context, username string) (model.AdminUser, error)
	UpdateLastLogin(ctx context.Context, uid uint, at time.Time) error
}

type LoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // access TTL 秒数
}

type RefreshResult struct {
	AccessToken  string
	RefreshToken string // 轮换语义：前端必须用新值覆盖旧值，旧值在墓碑 TTL 内会触发复用检测
	ExpiresIn    int64
}

// dummyPasswordHash 用于让"用户不存在"路径也跑一次 bcrypt 比较，让登录耗时
// 与"密码错误"路径接近，封堵用户名枚举侧信道。值本身不需保密，只需 bcrypt 合法。
const dummyPasswordHash = "$2a$10$C6UzMDM.H6dfI/f/IKcEeO6DGw4ZSLiZUj2Ip7yUpfI2KI2Zg7W6e"

func NewService(
	logger *log.Logger,
	jwtUtil *jwt.JWT,
	authRepo *Repository,
	users UserLookup,
	cfg *Config,
) *Service {
	return &Service{
		logger: logger,
		jwt:    jwtUtil,
		repo:   authRepo,
		users:  users,
		cfg:    cfg,
	}
}

type Service struct {
	logger *log.Logger
	jwt    *jwt.JWT
	repo   *Repository
	users  UserLookup
	cfg    *Config
}

// Login 安全契约：
//   - 失败路径必须对外不可区分"用户不存在 / 密码错误 / 已禁用"以外的细节，
//     避免用户名枚举（耗时与错误码双侧信道）；
//   - 成功路径必须同时落 refresh record + sessions 索引（由 LoginCreate 原子保证），
//     否则后续 RevokeAll 会漏吊销当前会话。
func (s *Service) Login(ctx context.Context, req *apiv1.LoginRequest) (*LoginResult, error) {
	user, err := s.users.GetAdminUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 让"不存在"路径与"密码错误"路径耗时同阶——封堵时序侧信道。
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
			return nil, apiv1.ErrUnauthorized
		}
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}

	if user.IsDisabled {
		s.logger.WithContext(ctx).Warn("auth.login_disabled",
			zap.Uint("uid", user.ID), zap.String("username", user.Username))
		return nil, apiv1.ErrUserDisabled
	}

	compareErr := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if compareErr != nil {
		if errors.Is(compareErr, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, apiv1.ErrUnauthorized
		}
		return nil, apiv1.ErrInternalServerError.WithCause(compareErr)
	}

	rt, sid, err := GenRT()
	if err != nil {
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}
	access, err := s.jwt.GenToken(user.ID, time.Now().Add(s.cfg.AccessTTL),
		map[string]any{"sid": sid})
	if err != nil {
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}

	exp := time.Now().Add(s.cfg.RefreshTTL).Unix()
	recordJSON, err := buildRefreshRecord(rt, user.ID, exp)
	if err != nil {
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}
	if err := s.repo.LoginCreate(ctx, LoginCreateParams{
		UID:           user.ID,
		SID:           sid,
		RecordJSON:    recordJSON,
		RefreshTTLSec: int(s.cfg.RefreshTTL.Seconds()),
		ExpTS:         exp,
	}); err != nil {
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}

	// 审计字段，失败仅记录——不能因审计写库失败阻塞登录主路径。
	if err := s.users.UpdateLastLogin(ctx, user.ID, time.Now()); err != nil {
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

// Refresh 安全契约：
//   - 必须轮换 RT（旧值同时进墓碑），否则失去复用检测能力；
//   - 检测到复用必须 RevokeAllUserSessions——单条吊销无法阻断攻击者已掌握的
//     其它会话；
//   - record 损坏路径不能宽松处理，否则等于给攻击者一个绕过窗口。
func (s *Service) Refresh(ctx context.Context, req *apiv1.RefreshRequest) (*RefreshResult, error) {
	sid, _, err := ParseRT(req.RefreshToken)
	if err != nil {
		// 不区分对外错误码，避免泄漏 RT 内部格式细节。
		return nil, apiv1.ErrUnauthorized
	}

	rec, err := s.repo.GetRefreshRecord(ctx, sid)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			// record 不存在时必须查墓碑：墓碑命中 = 复用，未命中 = 自然过期。
			tombUID, tombErr := s.repo.GetTombUID(ctx, sid)
			if tombErr == nil && tombUID > 0 {
				count, revokeErr := s.repo.RevokeAllUserSessions(ctx, tombUID)
				s.logger.WithContext(ctx).Warn("auth.reuse_detected",
					zap.Uint("uid", tombUID), zap.String("sid", sid),
					zap.String("via", "tomb"),
					zap.Int("revoked_count", count), zap.Error(revokeErr))
				return nil, apiv1.ErrRefreshReused
			}
			return nil, apiv1.ErrRefreshExpired
		}
		if errors.Is(err, ErrRefreshRecordCorrupted) {
			tombUID, tombErr := s.repo.GetTombUID(ctx, sid)
			if tombErr == nil && tombUID > 0 {
				count, revokeErr := s.repo.RevokeAllUserSessions(ctx, tombUID)
				s.logger.WithContext(ctx).Warn("auth.reuse_detected",
					zap.Uint("uid", tombUID), zap.String("sid", sid),
					zap.String("via", "corrupted_record_tomb"),
					zap.Int("revoked_count", count), zap.Error(revokeErr))
				return nil, apiv1.ErrRefreshReused
			}
			s.logger.WithContext(ctx).Warn("auth.refresh_record_corrupted",
				zap.String("sid", sid), zap.Error(err))
			return nil, apiv1.ErrRefreshReused
		}
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}
	uid := rec.UID

	newRT, newSID, err := GenRT()
	if err != nil {
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}
	newAccess, err := s.jwt.GenToken(uid, time.Now().Add(s.cfg.AccessTTL),
		map[string]any{"sid": newSID})
	if err != nil {
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}
	newExp := time.Now().Add(s.cfg.RefreshTTL).Unix()
	newRecord, err := buildRefreshRecord(newRT, uid, newExp)
	if err != nil {
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}

	err = s.repo.RotateRefresh(ctx, RotateParams{
		UID:           uid,
		OldSID:        sid,
		NewSID:        newSID,
		ExpectedHash:  HashRT(req.RefreshToken),
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

	case errors.Is(err, ErrRotateExpired):
		return nil, apiv1.ErrRefreshExpired

	case errors.Is(err, ErrRotateReused):
		// hash 不匹配 = RT 被复用/篡改：必须 RevokeAll，不能仅吊销当前 sid。
		count, revokeErr := s.repo.RevokeAllUserSessions(ctx, uid)
		s.logger.WithContext(ctx).Warn("auth.reuse_detected",
			zap.Uint("uid", uid), zap.String("sid", sid),
			zap.String("via", "lua"),
			zap.Int("revoked_count", count), zap.Error(revokeErr))
		return nil, apiv1.ErrRefreshReused

	default:
		return nil, apiv1.ErrInternalServerError.WithCause(err)
	}
}

// Logout 契约：
//   - 幂等——RT 格式非法、session 已不存在 都返回成功，让前端无脑清 token；
//   - 单 sid 范围——禁止连坐其他 session，否则破坏多 Tab 用户体验；
//   - 不依赖 access 有效性——access 过期后用户也必须能登出。
func (s *Service) Logout(ctx context.Context, req *apiv1.LogoutRequest) error {
	sid, _, parseErr := ParseRT(req.RefreshToken)
	if parseErr != nil {
		// 幂等契约：nilerr 是业务语义而非吞错。
		return nil //nolint:nilerr
	}
	rec, err := s.repo.GetRefreshRecord(ctx, sid)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return apiv1.ErrInternalServerError.WithCause(err)
	}
	if err := s.repo.DeleteSession(ctx, rec.UID, sid); err != nil &&
		!errors.Is(err, ErrSessionNotFound) {
		return apiv1.ErrInternalServerError.WithCause(err)
	}
	s.logger.WithContext(ctx).Info("auth.logout",
		zap.Uint("uid", rec.UID), zap.String("sid", sid))
	return nil
}

// RevokeAllUserSessions reason 是审计字段，约定取值：
// password_change / disable / delete / reuse_detected / admin_kick。
// 新增值需同步告警仪表盘的过滤规则。
func (s *Service) RevokeAllUserSessions(ctx context.Context, uid uint, reason string) (int, error) {
	count, err := s.repo.RevokeAllUserSessions(ctx, uid)
	s.logger.WithContext(ctx).Warn("auth.revoke_all",
		zap.Uint("uid", uid),
		zap.String("reason", reason),
		zap.Int("sid_count", count),
		zap.Error(err))
	if err != nil {
		return count, apiv1.ErrInternalServerError.WithCause(err)
	}
	return count, nil
}

func (s *Service) ListUserSessions(ctx context.Context, uid uint) ([]SessionInfo, error) {
	return s.repo.ListUserSessions(ctx, uid)
}

func (s *Service) KickSession(ctx context.Context, uid uint, sid string) error {
	if err := s.repo.DeleteSession(ctx, uid, sid); err != nil &&
		!errors.Is(err, ErrSessionNotFound) {
		return apiv1.ErrInternalServerError.WithCause(err)
	}
	s.logger.WithContext(ctx).Warn("auth.admin_kick",
		zap.Uint("uid", uid), zap.String("sid", sid))
	return nil
}

// buildRefreshRecord 数据契约：
//
//	th  = sha256(rt_raw) hex，原始 RT 永不落地；
//	uid = 用于 RevokeAll 路径回溯；
//	exp = 秒级失效戳，必须与 sessions ZSet 的 score 同值，否则索引清理会漏。
func buildRefreshRecord(rtRaw string, uid uint, exp int64) (string, error) {
	rec := RefreshRecord{
		TokenHash: HashRT(rtRaw),
		UID:       uid,
		Exp:       exp,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return "", fmt.Errorf("marshal refresh record: %w", err)
	}
	return string(b), nil
}
