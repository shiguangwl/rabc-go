package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"rabc-go/api/apiv1"
	"rabc-go/internal/auth"
	"rabc-go/internal/model"
	"rabc-go/internal/repository"
	"rabc-go/pkg/jwt"
	"rabc-go/pkg/log"
)

// stubAdminRepo 是仅满足 AuthService 测试所需子集的 AdminRepository stub。
//
// 设计 Why：AuthService 仅调用 GetAdminUserByUsername；其他方法不会触达。
// 嵌入 nil 接口让未实现方法的调用直接 panic，比手写返回 zero value 更安全
// （误触发立即暴露而非静默错误）。
type stubAdminRepo struct {
	repository.AdminRepository
	users map[string]model.AdminUser
}

func (s *stubAdminRepo) GetAdminUserByUsername(_ context.Context, username string) (model.AdminUser, error) {
	u, ok := s.users[username]
	if !ok {
		return model.AdminUser{}, gorm.ErrRecordNotFound
	}
	return u, nil
}

// UpdateLastLogin Login 成功路径调用：测试不关心持久化，仅吞掉，返回 nil。
func (*stubAdminRepo) UpdateLastLogin(_ context.Context, _ uint, _ time.Time) error {
	return nil
}

// newTestAuthService 构造一份 AuthService 测试桩 + 暴露 miniredis 供断言。
func newTestAuthService(t *testing.T, users ...model.AdminUser) (AuthService, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret-key-for-unit-test-only")
	j := jwt.NewJwt(conf)
	logger := log.NewLog(testLogConfig(t))

	adminRepo := &stubAdminRepo{users: make(map[string]model.AdminUser)}
	for _, u := range users {
		adminRepo.users[u.Username] = u
	}
	authRepo := repository.NewAuthRepository(rdb)
	authCfg := &auth.AuthConfig{
		AccessTTL:       30 * time.Minute,
		RefreshTTL:      168 * time.Hour,
		RotationTombTTL: 30 * time.Minute,
	}
	svc := NewAuthService(
		NewService(logger, j),
		authRepo, adminRepo, authCfg,
	)
	return svc, mr
}

func testLogConfig(t *testing.T) *viper.Viper {
	t.Helper()
	c := viper.New()
	c.Set("log.log_level", "error") // 测试期降噪
	c.Set("log.encoding", "console")
	return c
}

// mustHashPwd 生成 bcrypt 密码 hash，供测试构造 user 时使用。
func mustHashPwd(t *testing.T) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func TestAuthService_Login_Success(t *testing.T) {
	u := model.AdminUser{Username: "alice", Password: mustHashPwd(t)}
	u.ID = 1
	svc, mr := newTestAuthService(t, u)

	result, err := svc.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice", Password: "secret123",
	})
	if err != nil {
		t.Fatalf("Login err = %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" || result.ExpiresIn <= 0 {
		t.Fatalf("result missing fields: %+v", result)
	}
	if len(result.RefreshToken) != repository.RTEncodedLen {
		t.Fatalf("RT length = %d, want %d", len(result.RefreshToken), repository.RTEncodedLen)
	}
	// Redis 应已写入 refresh_key + sessions ZSet
	sid, _, _ := repository.ParseRT(result.RefreshToken)
	if !mr.Exists("auth:refresh:" + sid) {
		t.Fatalf("refresh key not written")
	}
}

func TestAuthService_Login_UserDisabledReturns403(t *testing.T) {
	u := model.AdminUser{
		Username: "alice", Password: mustHashPwd(t),
		IsDisabled: true,
	}
	u.ID = 1
	svc, _ := newTestAuthService(t, u)

	_, err := svc.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice", Password: "secret123",
	})
	if !errors.Is(err, apiv1.ErrUserDisabled) {
		t.Fatalf("err = %v, want ErrUserDisabled", err)
	}
}

func TestAuthService_Login_WrongPasswordReturns401(t *testing.T) {
	u := model.AdminUser{Username: "alice", Password: mustHashPwd(t)}
	u.ID = 1
	svc, _ := newTestAuthService(t, u)

	_, err := svc.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice", Password: "WRONG",
	})
	if !errors.Is(err, apiv1.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestAuthService_Login_UserNotFoundReturns401(t *testing.T) {
	svc, _ := newTestAuthService(t) // 无用户

	_, err := svc.Login(context.Background(), &apiv1.LoginRequest{
		Username: "ghost", Password: "anything",
	})
	if !errors.Is(err, apiv1.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestAuthService_Refresh_SuccessRotation(t *testing.T) {
	u := model.AdminUser{Username: "alice", Password: mustHashPwd(t)}
	u.ID = 1
	svc, mr := newTestAuthService(t, u)

	loginResult, err := svc.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice", Password: "secret123",
	})
	if err != nil {
		t.Fatal(err)
	}
	oldRT := loginResult.RefreshToken
	oldSID, _, _ := repository.ParseRT(oldRT)

	refreshResult, err := svc.Refresh(context.Background(), &apiv1.RefreshRequest{
		RefreshToken: oldRT,
	})
	if err != nil {
		t.Fatalf("Refresh err = %v", err)
	}
	if refreshResult.RefreshToken == oldRT {
		t.Fatal("新 RT 应不同于旧 RT（轮换语义）")
	}
	if !mr.Exists("auth:refresh:tomb:" + oldSID) {
		t.Fatal("旧 sid 墓碑应已写入")
	}
	if mr.Exists("auth:refresh:" + oldSID) {
		t.Fatal("旧 refresh key 应已删除")
	}
}

func TestAuthService_Refresh_ExpiredReturns401(t *testing.T) {
	svc, _ := newTestAuthService(t)

	// 构造一份合法格式的 RT 但 Redis 中无对应 key
	bogus, _, _ := repository.GenRT()
	_, err := svc.Refresh(context.Background(), &apiv1.RefreshRequest{RefreshToken: bogus})
	if !errors.Is(err, apiv1.ErrRefreshExpired) {
		t.Fatalf("err = %v, want ErrRefreshExpired", err)
	}
}

func TestAuthService_Refresh_FormatErrorReturns401(t *testing.T) {
	svc, _ := newTestAuthService(t)
	_, err := svc.Refresh(context.Background(), &apiv1.RefreshRequest{RefreshToken: "!!!bad-rt!!!"})
	if !errors.Is(err, apiv1.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestAuthService_Refresh_ReusedTriggersRevokeAll(t *testing.T) {
	u := model.AdminUser{Username: "alice", Password: mustHashPwd(t)}
	u.ID = 1
	svc, mr := newTestAuthService(t, u)

	loginResult, err := svc.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice", Password: "secret123",
	})
	if err != nil {
		t.Fatal(err)
	}
	oldRT := loginResult.RefreshToken

	// 第一次刷新成功（合法路径）
	if _, refreshErr := svc.Refresh(context.Background(), &apiv1.RefreshRequest{RefreshToken: oldRT}); refreshErr != nil {
		t.Fatal(refreshErr)
	}

	// 第二次用旧 RT（墓碑应命中 → reused）
	_, err = svc.Refresh(context.Background(), &apiv1.RefreshRequest{RefreshToken: oldRT})
	if !errors.Is(err, apiv1.ErrRefreshReused) {
		t.Fatalf("err = %v, want ErrRefreshReused", err)
	}

	// 复用检测应触发 RevokeAll：该 user 全部 session 应被清空
	members, _ := mr.ZMembers("auth:user:1:sessions")
	if len(members) != 0 {
		t.Fatalf("sessions 应被 RevokeAll 清空，实际 %v", members)
	}
}

func TestAuthService_Refresh_CorruptedRecordReturnsReused(t *testing.T) {
	u := model.AdminUser{Username: "alice", Password: mustHashPwd(t)}
	u.ID = 1
	svc, mr := newTestAuthService(t, u)

	loginResult, err := svc.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice", Password: "secret123",
	})
	if err != nil {
		t.Fatal(err)
	}
	sid, _, _ := repository.ParseRT(loginResult.RefreshToken)
	mr.Set("auth:refresh:"+sid, "{bad-json")

	_, err = svc.Refresh(context.Background(), &apiv1.RefreshRequest{RefreshToken: loginResult.RefreshToken})
	if !errors.Is(err, apiv1.ErrRefreshReused) {
		t.Fatalf("err = %v, want ErrRefreshReused", err)
	}
}

func TestAuthService_Logout_RemovesSession(t *testing.T) {
	u := model.AdminUser{Username: "alice", Password: mustHashPwd(t)}
	u.ID = 1
	svc, mr := newTestAuthService(t, u)

	loginResult, _ := svc.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice", Password: "secret123",
	})
	sid, _, _ := repository.ParseRT(loginResult.RefreshToken)

	err := svc.Logout(context.Background(), &apiv1.LogoutRequest{
		RefreshToken: loginResult.RefreshToken,
	})
	if err != nil {
		t.Fatalf("Logout err = %v", err)
	}
	if mr.Exists("auth:refresh:" + sid) {
		t.Fatal("Logout 后 refresh_key 应删除")
	}
}

func TestAuthService_Logout_AlreadyGoneIsIdempotent(t *testing.T) {
	svc, _ := newTestAuthService(t)
	bogus, _, _ := repository.GenRT()
	// 不应返错（幂等）
	if err := svc.Logout(context.Background(), &apiv1.LogoutRequest{RefreshToken: bogus}); err != nil {
		t.Fatalf("Logout 不存在 session 应幂等，err = %v", err)
	}
}

func TestAuthService_Logout_MalformedRTIsIdempotent(t *testing.T) {
	svc, _ := newTestAuthService(t)
	// 格式错的 RT 也应幂等成功（前端清 token 即可）
	if err := svc.Logout(context.Background(), &apiv1.LogoutRequest{RefreshToken: "!!!"}); err != nil {
		t.Fatalf("Logout malformed RT 应幂等，err = %v", err)
	}
}

func TestAuthService_RevokeAllUserSessions_ClearsAllAndLogs(t *testing.T) {
	u := model.AdminUser{Username: "alice", Password: mustHashPwd(t)}
	u.ID = 1
	svc, mr := newTestAuthService(t, u)

	// 模拟两次登录（不同 Tab）
	_, _ = svc.Login(context.Background(), &apiv1.LoginRequest{Username: "alice", Password: "secret123"})
	_, _ = svc.Login(context.Background(), &apiv1.LoginRequest{Username: "alice", Password: "secret123"})

	count, err := svc.RevokeAllUserSessions(context.Background(), 1, "test_reason")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("revoked count = %d, want 2", count)
	}
	members, _ := mr.ZMembers("auth:user:1:sessions")
	if len(members) != 0 {
		t.Fatalf("RevokeAll 后应清空，实际 %v", members)
	}
}

func TestAuthService_ListAndKick(t *testing.T) {
	u := model.AdminUser{Username: "alice", Password: mustHashPwd(t)}
	u.ID = 1
	svc, _ := newTestAuthService(t, u)

	loginResult, _ := svc.Login(context.Background(), &apiv1.LoginRequest{
		Username: "alice", Password: "secret123",
	})
	sid, _, _ := repository.ParseRT(loginResult.RefreshToken)

	sessions, err := svc.ListUserSessions(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].SID != sid {
		t.Fatalf("sessions = %+v, want 1 session with sid=%s", sessions, sid)
	}

	if err := svc.KickSession(context.Background(), 1, sid); err != nil {
		t.Fatal(err)
	}
	sessions2, _ := svc.ListUserSessions(context.Background(), 1)
	if len(sessions2) != 0 {
		t.Fatalf("Kick 后 sessions = %+v, want empty", sessions2)
	}

	// 避免 logger 等 unused
	_ = zap.String
}
