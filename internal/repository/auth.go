// Package repository 中的 AuthRepository 负责 Refresh Token 与 Session
// 在 Redis 中的存储与原子操作。
//
// Refresh Token 二进制布局（32B raw → base64.RawURLEncoding 43 字符）：
//
//	[0..16)   session_id   16B（crypto/rand）
//	[16..32)  nonce        16B（crypto/rand）
//
// RT 本身不含 uid，uid 只能从 Redis 中 auth:refresh:{sid} 记录的 JSON 字段读取。
//
// 关键 Redis Key：
//
//	auth:refresh:{sid}        String JSON {th, uid, exp, ua, ip}        TTL=refresh_ttl
//	auth:refresh:tomb:{sid}   String "1"                                  TTL=rotation_tomb_ttl
//	auth:user:{uid}:sessions  ZSet member=sid score=exp_ts               无 key TTL
//	auth:jti:blacklist:{jti}  String "1"                                  TTL=access 剩余
package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "embed"

	"github.com/redis/go-redis/v9"
)

// RT 二进制布局长度常量（详见 package doc）。
//
// 字段长度约束：
//   - sid 16B（128 bit）让攻击者枚举 sid 概率忽略不计；
//   - rand 16B（128 bit）让 hash(rt) 输入空间够大不可猜测。
//
// 总 32B raw → base64.RawURLEncoding 实际输出 43 字符（ceil(32 * 4 / 3)）。
const (
	rtSIDLen  = 16
	rtRandLen = 16
	rtRawLen  = rtSIDLen + rtRandLen // 32

	// RTEncodedLen 是 base64.RawURLEncoding(32B) 后的字符数。
	RTEncodedLen = 43
)

// Lua 脚本通过 //go:embed 静态嵌入二进制，避免运行时依赖文件路径或加载顺序，
// 同时让 IDE 与静态检查器能直接识别脚本文件位置。
//
// 注：embed 路径相对当前 Go 文件目录；脚本在 internal/repository/lua/ 下。
//
//go:embed lua/rotate.lua
var rotateScriptSource string

//go:embed lua/loginCreate.lua
var loginCreateScriptSource string

var (
	rotateScript      = redis.NewScript(rotateScriptSource)
	loginCreateScript = redis.NewScript(loginCreateScriptSource)
)

// AuthRepository 是 Auth 子系统对 Redis 的访问入口。
//
// 所有 Refresh / Session / JTI 相关 Redis 操作必须从这里进入，避免 service
// 层散落 Redis key 规则。
type AuthRepository interface {
	// LoginCreate 原子创建新会话：SET refresh_key + ZADD sessions。
	LoginCreate(ctx context.Context, p LoginCreateParams) error

	// RotateRefresh 原子轮换 Refresh Token，返回 3 种错误码之一：
	//   nil                 success
	//   ErrRotateExpired    自然过期
	//   ErrRotateReused     hash 不匹配 / 墓碑命中 / JSON 损坏（合并视作攻击）
	RotateRefresh(ctx context.Context, p RotateParams) error

	// DeleteSession 删除单个 sid 对应的 refresh key 与 sessions 索引项。
	// 用于 Logout（用户主动登出）；不连坐 user 其它 session。
	DeleteSession(ctx context.Context, uid uint, sid string) error

	// GetRefreshRecord 取 sid 对应的 record JSON 并反序列化。
	// 用于 service 层验证 hash + 提取 uid。
	GetRefreshRecord(ctx context.Context, sid string) (*RefreshRecord, error)

	// GetTombUID 取 sid 对应墓碑 value（uid 字符串），返回 uint。
	// service 层在 GetRefreshRecord 返回 ErrSessionNotFound 时调用本方法判断：
	//   - 存在 → tomb hit（reused 路径），返 uid 触发 RevokeAllUserSessions
	//   - 不存在 → 真过期（自然过期路径，仅返 401 不连坐）
	GetTombUID(ctx context.Context, sid string) (uint, error)

	// RevokeAllUserSessions 用 Go pipeline 批量删除该 user 全部 session。
	// 最终一致语义（部分失败仅 warn 不报错），返回被删除的 sid 数量。
	RevokeAllUserSessions(ctx context.Context, uid uint) (int, error)

	// ListUserSessions 列出该 user 当前活跃 session（按 exp 升序）。
	// 用于管理端会话查看。
	ListUserSessions(ctx context.Context, uid uint) ([]SessionInfo, error)

	// BlacklistJTI 把指定 jti 加入紧急吊销黑名单，TTL ≤ access 剩余时间。
	BlacklistJTI(ctx context.Context, jti string, ttl time.Duration) error

	// IsJTIBlacklisted 检查 jti 是否在黑名单中（兜底吊销路径）。
	IsJTIBlacklisted(ctx context.Context, jti string) (bool, error)
}

// LoginCreateParams 登录创建参数。
type LoginCreateParams struct {
	UID           uint
	SID           string
	RecordJSON    string
	RefreshTTLSec int
	ExpTS         int64
}

// RotateParams 轮换参数。
type RotateParams struct {
	UID           uint
	OldSID        string
	NewSID        string
	ExpectedHash  string
	NewRecordJSON string
	RefreshTTLSec int
	TombTTLSec    int
	NewExpTS      int64
}

// RefreshRecord 是 Redis 中 auth:refresh:{sid} 的 JSON 反序列化结构。
//
// record JSON 字段名必须集中在本类型维护；`th` 是 sha256(rt_raw) 的 hex，
// 不存原始 RT。
type RefreshRecord struct {
	TokenHash string `json:"th"`
	UID       uint   `json:"uid"`
	Exp       int64  `json:"exp"`
	UAHash    string `json:"ua,omitempty"`
	IP        string `json:"ip,omitempty"`
}

// SessionInfo 管理端会话列表项。
type SessionInfo struct {
	SID string
	Exp int64
}

// Sentinel errors。service 层用 errors.Is 判定路径。
var (
	ErrRotateExpired = errors.New("auth: refresh expired")
	ErrRotateReused  = errors.New("auth: refresh reused or corrupted")

	// ErrRefreshFormat 标记 RT 字符串格式异常（base64 解码失败 / 长度不符）。
	ErrRefreshFormat = errors.New("auth: refresh token format invalid")

	// ErrRefreshRecordCorrupted 标记 Redis 中 refresh record 无法解码。
	// 这类数据异常按 reused 路径处理，避免攻击者绕过复用检测。
	ErrRefreshRecordCorrupted = errors.New("auth: refresh record corrupted")

	// ErrSessionNotFound DeleteSession / GetRefreshRecord 找不到 sid。
	// 用于 service 层区分幂等登出、自然过期和复用检测路径。
	ErrSessionNotFound = errors.New("auth: session not found")
)

// authRepo 是 AuthRepository 的 Redis 实现。
type authRepo struct {
	rdb *redis.Client
}

// NewAuthRepository 构造 AuthRepository；wire 用。
func NewAuthRepository(rdb *redis.Client) AuthRepository {
	return &authRepo{rdb: rdb}
}

// ParseRT 解析 Refresh Token base64url 字符串，返回 sid 与 rand。
//
// 严格不变量：长度必须等于 rtRawLen=32B；任何长度不符或 base64 错误直接返回
// ErrRefreshFormat，service 层据此判 401。
func ParseRT(raw string) (sid string, rnd string, err error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", "", fmt.Errorf("%w: base64 decode: %v", ErrRefreshFormat, err)
	}
	if len(decoded) != rtRawLen {
		return "", "", fmt.Errorf("%w: expected %dB, got %dB",
			ErrRefreshFormat, rtRawLen, len(decoded))
	}
	sid = hex.EncodeToString(decoded[:rtSIDLen])
	rnd = hex.EncodeToString(decoded[rtSIDLen:])
	return sid, rnd, nil
}

// GenRT 生成新的 Refresh Token raw 字符串与对应 sid。
//
// sid 与 rand 必须各保留 16B 不可预测性；sid 使用 hex 作为 Redis key 后缀。
func GenRT() (raw string, sid string, err error) {
	buf := make([]byte, rtRawLen)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("crypto/rand read: %w", err)
	}
	sid = hex.EncodeToString(buf[:rtSIDLen])
	return base64.RawURLEncoding.EncodeToString(buf), sid, nil
}

// HashRT 计算 raw 字符串的 sha256 hex（小写）。
// service 层用本函数生成 record.th 与 RotateRefresh 入参 ExpectedHash。
func HashRT(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Redis key 拼装放到内部辅助函数，统一格式避免散落硬编码。
func refreshKey(sid string) string      { return "auth:refresh:" + sid }
func tombKey(sid string) string         { return "auth:refresh:tomb:" + sid }
func sessionsKey(uid uint) string       { return fmt.Sprintf("auth:user:%d:sessions", uid) }
func jtiBlacklistKey(jti string) string { return "auth:jti:blacklist:" + jti }

// LoginCreate 执行 loginCreate.lua 原子写入。
func (r *authRepo) LoginCreate(ctx context.Context, p LoginCreateParams) error {
	res, err := loginCreateScript.Run(ctx, r.rdb,
		[]string{refreshKey(p.SID), sessionsKey(p.UID)},
		p.RecordJSON, p.RefreshTTLSec, p.SID, p.ExpTS,
	).Slice()
	if err != nil {
		return fmt.Errorf("redis eval loginCreate: %w", err)
	}
	if len(res) == 0 {
		return fmt.Errorf("redis eval loginCreate: empty result")
	}
	code, _ := res[0].(int64)
	if code != 0 {
		return fmt.Errorf("redis eval loginCreate: unexpected code %d", code)
	}
	return nil
}

// RotateRefresh 执行 rotate.lua 原子轮换。
//
// 错误码映射（与脚本 doc 对齐）：
//
//	0 → nil success
//	1 → ErrRotateExpired
//	2 → ErrRotateReused（hash 不匹配 / 墓碑命中 / JSON 损坏 全归此）
//
// 墓碑 value 必须写入 uid 字符串，service 层依赖它在 tomb hit 路径下吊销会话。
func (r *authRepo) RotateRefresh(ctx context.Context, p RotateParams) error {
	res, err := rotateScript.Run(ctx, r.rdb,
		[]string{
			refreshKey(p.OldSID), tombKey(p.OldSID),
			refreshKey(p.NewSID), sessionsKey(p.UID),
		},
		p.ExpectedHash, p.NewRecordJSON, p.RefreshTTLSec, p.TombTTLSec,
		p.NewSID, p.NewExpTS, p.OldSID,
		fmt.Sprintf("%d", p.UID), // 墓碑 value 必须是 uid 字符串。
	).Slice()
	if err != nil {
		return fmt.Errorf("redis eval rotate: %w", err)
	}
	if len(res) == 0 {
		return fmt.Errorf("redis eval rotate: empty result")
	}
	code, _ := res[0].(int64)
	switch code {
	case 0:
		return nil
	case 1:
		return ErrRotateExpired
	case 2:
		return ErrRotateReused
	default:
		return fmt.Errorf("redis eval rotate: unknown code %d", code)
	}
}

// DeleteSession 删除单个 session（Logout 场景）。
//
// 返回 ErrSessionNotFound 让 service 层按调用场景处理幂等语义。
func (r *authRepo) DeleteSession(ctx context.Context, uid uint, sid string) error {
	pipe := r.rdb.Pipeline()
	delCmd := pipe.Del(ctx, refreshKey(sid))
	pipe.ZRem(ctx, sessionsKey(uid), sid)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis pipeline delete session: %w", err)
	}
	if delCmd.Val() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// GetRefreshRecord 取 sid 对应的 record JSON。
func (r *authRepo) GetRefreshRecord(ctx context.Context, sid string) (*RefreshRecord, error) {
	raw, err := r.rdb.Get(ctx, refreshKey(sid)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("redis get refresh record: %w", err)
	}
	var rec RefreshRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRefreshRecordCorrupted, err)
	}
	return &rec, nil
}

// GetTombUID 反查墓碑中存的 uid。
//
// 墓碑 value 必须是 uid 字符串，否则 tomb hit 路径无法定位需要吊销的用户。
//
// 返回 ErrSessionNotFound 表示墓碑不存在（真自然过期路径）。
func (r *authRepo) GetTombUID(ctx context.Context, sid string) (uint, error) {
	raw, err := r.rdb.Get(ctx, tombKey(sid)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrSessionNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("redis get tomb: %w", err)
	}
	var uid uint
	if _, err := fmt.Sscanf(raw, "%d", &uid); err != nil {
		return 0, fmt.Errorf("parse tomb uid %q: %w", raw, err)
	}
	return uid, nil
}

// RevokeAllUserSessions 用 Go pipeline 批量删除（不用 Lua，避免 Cluster CROSSSLOT）。
//
// 约束：
//   - revoke 是最终一致操作，部分 sid DEL 失败不回滚。
//   - 多 key 删除使用 Go pipeline，避免 Lua 在 Cluster 部署下触发 CROSSSLOT。
//
// 部分失败策略：pipeline.Exec 报错时返回错误但不回滚（最终一致）；service 层
// 调用方应当 warn + metric 告警，不阻塞业务返回。
func (r *authRepo) RevokeAllUserSessions(ctx context.Context, uid uint) (int, error) {
	key := sessionsKey(uid)
	sids, err := r.rdb.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return 0, fmt.Errorf("zrange sessions: %w", err)
	}
	if len(sids) == 0 {
		return 0, nil
	}
	pipe := r.rdb.Pipeline()
	for _, sid := range sids {
		pipe.Del(ctx, refreshKey(sid))
	}
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		// 返回错误供 service 层告警；已删除的 key 不做回滚。
		return len(sids), fmt.Errorf("redis pipeline revoke: %w", err)
	}
	return len(sids), nil
}

// ListUserSessions 列出活跃 session。
//
// 用 ZRANGEBYSCORE now +inf 过滤过期 sid（虽然 refresh_key 自动 TTL 过期，但
// sessions ZSet 没有 key TTL，会有"僵尸 sid"，本方法侧把它们排除）。
// 副作用：顺便触发 ZREMRANGEBYSCORE 清理（异步式 best-effort，不阻塞返回）。
func (r *authRepo) ListUserSessions(ctx context.Context, uid uint) ([]SessionInfo, error) {
	key := sessionsKey(uid)
	now := time.Now().Unix()
	zs, err := r.rdb.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", now),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("zrangebyscore sessions: %w", err)
	}
	out := make([]SessionInfo, 0, len(zs))
	for _, z := range zs {
		member, _ := z.Member.(string)
		out = append(out, SessionInfo{SID: member, Exp: int64(z.Score)})
	}
	// 清理过期索引是维护动作，失败不影响本次列表结果。
	_ = r.rdb.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("(%d", now)).Err()
	return out, nil
}

// BlacklistJTI 把 jti 加入紧急吊销黑名单。
//
// JTI 黑名单只用于紧急吊销场景，TTL 必须小于等于 access 剩余生命周期，
// 避免黑名单长期膨胀。
func (r *authRepo) BlacklistJTI(ctx context.Context, jti string, ttl time.Duration) error {
	if strings.TrimSpace(jti) == "" {
		return fmt.Errorf("jti must not be empty")
	}
	if ttl <= 0 {
		return fmt.Errorf("blacklist ttl must be positive, got %v", ttl)
	}
	return r.rdb.Set(ctx, jtiBlacklistKey(jti), "1", ttl).Err()
}

// IsJTIBlacklisted 查询 jti 是否在黑名单。
func (r *authRepo) IsJTIBlacklisted(ctx context.Context, jti string) (bool, error) {
	n, err := r.rdb.Exists(ctx, jtiBlacklistKey(jti)).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists jti blacklist: %w", err)
	}
	return n > 0, nil
}
