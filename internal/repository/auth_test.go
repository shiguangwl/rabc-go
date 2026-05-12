package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestAuthRepo 用 miniredis 起一个隔离的 Redis 测试实例。
// 每个测试独立 miniredis，避免 key 残留影响其他用例。
func newTestAuthRepo(t *testing.T) (*authRepo, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return &authRepo{rdb: rdb}, mr
}

func TestParseRTAndGenRTRoundTrip(t *testing.T) {
	raw, sid, err := GenRT()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != RTEncodedLen {
		t.Fatalf("RT length = %d, want %d", len(raw), RTEncodedLen)
	}
	if len(sid) != 2*rtSIDLen {
		t.Fatalf("sid hex length = %d, want %d", len(sid), 2*rtSIDLen)
	}
	gotSID, _, err := ParseRT(raw)
	if err != nil {
		t.Fatalf("ParseRT() error = %v", err)
	}
	if gotSID != sid {
		t.Fatalf("parsed sid = %q, want %q", gotSID, sid)
	}
}

func TestParseRTRejectsLengthVariations(t *testing.T) {
	cases := []struct {
		name string
		size int
	}{
		{"too_short_0", 0},
		{"too_short_23", 23},
		{"too_long_25", 25},
		{"too_long_56", 56},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, tc.size)
			raw := base64.RawURLEncoding.EncodeToString(buf)
			_, _, err := ParseRT(raw)
			if !errors.Is(err, ErrRefreshFormat) {
				t.Fatalf("ParseRT(%dB) error = %v, want ErrRefreshFormat", tc.size, err)
			}
		})
	}
}

func TestParseRTRejectsInvalidBase64(t *testing.T) {
	_, _, err := ParseRT("!!!not-valid-base64!!!")
	if !errors.Is(err, ErrRefreshFormat) {
		t.Fatalf("ParseRT(invalid base64) error = %v, want ErrRefreshFormat", err)
	}
}

func TestHashRTDeterministic(t *testing.T) {
	if HashRT("abc") != HashRT("abc") {
		t.Fatal("HashRT not deterministic")
	}
	if HashRT("abc") == HashRT("abd") {
		t.Fatal("HashRT collision on tiny input")
	}
}

// helper: 构造一份 record JSON 写入 Redis，模拟登录后的状态
func writeRefreshRecord(t *testing.T, mr *miniredis.Miniredis, uid uint, sid string, hash string, exp int64) {
	t.Helper()
	rec := RefreshRecord{TokenHash: hash, UID: uid, Exp: exp}
	b, _ := json.Marshal(rec)
	mr.Set(refreshKey(sid), string(b))
	// sessions ZSet
	mr.ZAdd(sessionsKey(uid), float64(exp), sid)
}

func TestLoginCreateAtomic(t *testing.T) {
	r, mr := newTestAuthRepo(t)
	rec := RefreshRecord{TokenHash: "hash1", UID: 7, Exp: 9999999999}
	b, _ := json.Marshal(rec)

	err := r.LoginCreate(context.Background(), LoginCreateParams{
		UID:           7,
		SID:           "sid-aaa",
		RecordJSON:    string(b),
		RefreshTTLSec: 3600,
		ExpTS:         9999999999,
	})
	if err != nil {
		t.Fatalf("LoginCreate error = %v", err)
	}
	// 验证 refresh_key 与 sessions ZSet 都已写入
	if !mr.Exists("auth:refresh:sid-aaa") {
		t.Fatalf("refresh_key not written")
	}
	members, err := mr.ZMembers(sessionsKey(7))
	if err != nil || len(members) != 1 || members[0] != "sid-aaa" {
		t.Fatalf("sessions ZSet members = %v, want [sid-aaa]; err=%v", members, err)
	}
}

func TestRotateRefreshSuccessPath(t *testing.T) {
	r, mr := newTestAuthRepo(t)
	hash := "hashOld"
	writeRefreshRecord(t, mr, 7, "sid-old", hash, 9999999999)

	newRec := RefreshRecord{TokenHash: "hashNew", UID: 7, Exp: 9999999999}
	nb, _ := json.Marshal(newRec)
	err := r.RotateRefresh(context.Background(), RotateParams{
		UID:           7,
		OldSID:        "sid-old",
		NewSID:        "sid-new",
		ExpectedHash:  hash,
		NewRecordJSON: string(nb),
		RefreshTTLSec: 3600,
		TombTTLSec:    1800,
		NewExpTS:      9999999999,
	})
	if err != nil {
		t.Fatalf("RotateRefresh success path err = %v", err)
	}
	if mr.Exists("auth:refresh:sid-old") {
		t.Fatal("旧 refresh_key 应已删除")
	}
	if !mr.Exists("auth:refresh:sid-new") {
		t.Fatal("新 refresh_key 应已写入")
	}
	if !mr.Exists("auth:refresh:tomb:sid-old") {
		t.Fatal("旧 sid 墓碑应已写入")
	}
}

func TestRotateRefreshExpiredKey(t *testing.T) {
	r, _ := newTestAuthRepo(t)
	// 不写入 refresh_key 也不写墓碑 → 期望 ErrRotateExpired
	err := r.RotateRefresh(context.Background(), RotateParams{
		UID:           7,
		OldSID:        "sid-old",
		NewSID:        "sid-new",
		ExpectedHash:  "anything",
		NewRecordJSON: `{"th":"x"}`,
		RefreshTTLSec: 3600,
		TombTTLSec:    1800,
		NewExpTS:      9999999999,
	})
	if !errors.Is(err, ErrRotateExpired) {
		t.Fatalf("err = %v, want ErrRotateExpired", err)
	}
}

func TestRotateRefreshTombHitReturnsReused(t *testing.T) {
	r, mr := newTestAuthRepo(t)
	// 仅写墓碑（不写 refresh_key） → 期望 ErrRotateReused
	mr.Set(tombKey("sid-old"), "1")
	err := r.RotateRefresh(context.Background(), RotateParams{
		UID: 7, OldSID: "sid-old", NewSID: "sid-new",
		ExpectedHash:  "anything",
		NewRecordJSON: `{"th":"x"}`,
		RefreshTTLSec: 3600, TombTTLSec: 1800, NewExpTS: 9999999999,
	})
	if !errors.Is(err, ErrRotateReused) {
		t.Fatalf("tomb hit err = %v, want ErrRotateReused", err)
	}
}

func TestRotateRefreshHashMismatchReturnsReused(t *testing.T) {
	r, mr := newTestAuthRepo(t)
	writeRefreshRecord(t, mr, 7, "sid-old", "real-hash", 9999999999)
	err := r.RotateRefresh(context.Background(), RotateParams{
		UID: 7, OldSID: "sid-old", NewSID: "sid-new",
		ExpectedHash:  "wrong-hash",
		NewRecordJSON: `{"th":"x"}`,
		RefreshTTLSec: 3600, TombTTLSec: 1800, NewExpTS: 9999999999,
	})
	if !errors.Is(err, ErrRotateReused) {
		t.Fatalf("hash mismatch err = %v, want ErrRotateReused", err)
	}
}

func TestRotateRefreshCorruptedJSONReturnsReused(t *testing.T) {
	r, mr := newTestAuthRepo(t)
	mr.Set(refreshKey("sid-old"), "not-a-valid-json")
	err := r.RotateRefresh(context.Background(), RotateParams{
		UID: 7, OldSID: "sid-old", NewSID: "sid-new",
		ExpectedHash:  "anything",
		NewRecordJSON: `{"th":"x"}`,
		RefreshTTLSec: 3600, TombTTLSec: 1800, NewExpTS: 9999999999,
	})
	if !errors.Is(err, ErrRotateReused) {
		t.Fatalf("corrupted json err = %v, want ErrRotateReused", err)
	}
}

func TestDeleteSessionRemovesRefreshAndZSet(t *testing.T) {
	r, mr := newTestAuthRepo(t)
	writeRefreshRecord(t, mr, 7, "sid-foo", "h", 9999999999)
	err := r.DeleteSession(context.Background(), 7, "sid-foo")
	if err != nil {
		t.Fatalf("DeleteSession err = %v", err)
	}
	if mr.Exists(refreshKey("sid-foo")) {
		t.Fatal("refresh_key 应已删除")
	}
	members, _ := mr.ZMembers(sessionsKey(7))
	if len(members) != 0 {
		t.Fatalf("sessions members = %v, want empty", members)
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	r, _ := newTestAuthRepo(t)
	err := r.DeleteSession(context.Background(), 7, "no-such-sid")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("err = %v, want ErrSessionNotFound", err)
	}
}

func TestGetRefreshRecordRoundTrip(t *testing.T) {
	r, mr := newTestAuthRepo(t)
	writeRefreshRecord(t, mr, 42, "sid-x", "hashY", 1700000000)
	rec, err := r.GetRefreshRecord(context.Background(), "sid-x")
	if err != nil {
		t.Fatalf("GetRefreshRecord err = %v", err)
	}
	if rec.UID != 42 || rec.TokenHash != "hashY" || rec.Exp != 1700000000 {
		t.Fatalf("rec = %+v, want UID=42 TokenHash=hashY Exp=1700000000", rec)
	}
}

func TestGetRefreshRecordMissing(t *testing.T) {
	r, _ := newTestAuthRepo(t)
	_, err := r.GetRefreshRecord(context.Background(), "no-such-sid")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("err = %v, want ErrSessionNotFound", err)
	}
}

func TestRevokeAllUserSessionsClearsAllKeys(t *testing.T) {
	r, mr := newTestAuthRepo(t)
	writeRefreshRecord(t, mr, 7, "sid-a", "h1", 9999999999)
	writeRefreshRecord(t, mr, 7, "sid-b", "h2", 9999999999)
	writeRefreshRecord(t, mr, 7, "sid-c", "h3", 9999999999)

	n, err := r.RevokeAllUserSessions(context.Background(), 7)
	if err != nil {
		t.Fatalf("RevokeAll err = %v", err)
	}
	if n != 3 {
		t.Fatalf("revoked count = %d, want 3", n)
	}
	for _, sid := range []string{"sid-a", "sid-b", "sid-c"} {
		if mr.Exists(refreshKey(sid)) {
			t.Fatalf("refresh_key %s 仍存在", sid)
		}
	}
	if mr.Exists(sessionsKey(7)) {
		t.Fatal("sessions key 仍存在")
	}
}

func TestRevokeAllUserSessionsNoSessions(t *testing.T) {
	r, _ := newTestAuthRepo(t)
	n, err := r.RevokeAllUserSessions(context.Background(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("revoked count = %d, want 0", n)
	}
}

func TestListUserSessionsReturnsActiveOnly(t *testing.T) {
	r, mr := newTestAuthRepo(t)
	now := time.Now().Unix()
	// 1 个未过期 + 1 个已过期（exp < now）
	writeRefreshRecord(t, mr, 7, "active", "h1", now+3600)
	mr.ZAdd(sessionsKey(7), float64(now-3600), "stale")

	got, err := r.ListUserSessions(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SID != "active" {
		t.Fatalf("got = %+v, want only [active]", got)
	}
}

func TestBlacklistJTIRoundTrip(t *testing.T) {
	r, _ := newTestAuthRepo(t)
	err := r.BlacklistJTI(context.Background(), "jti-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	hit, err := r.IsJTIBlacklisted(context.Background(), "jti-1")
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Fatal("应命中黑名单")
	}
	miss, _ := r.IsJTIBlacklisted(context.Background(), "jti-2")
	if miss {
		t.Fatal("非黑名单 jti 不应命中")
	}
}

func TestBlacklistJTIRejectsBadInput(t *testing.T) {
	r, _ := newTestAuthRepo(t)
	if err := r.BlacklistJTI(context.Background(), "", time.Minute); err == nil {
		t.Fatal("空 jti 应返错")
	}
	if err := r.BlacklistJTI(context.Background(), "jti", 0); err == nil {
		t.Fatal("零 TTL 应返错")
	}
}
