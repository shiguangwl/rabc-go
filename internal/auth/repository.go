// Package auth 是认证子域：双 Token 颁发与轮换、复用检测、主动登出、
// 会话吊销与 JTI 黑名单。
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
//	auth:refresh:tomb:{sid}   String "<uid>"                              TTL=rotation_tomb_ttl
//	auth:user:{uid}:sessions  ZSet member=sid score=exp_ts               无 key TTL
//	auth:jti:blacklist:{jti}  String "1"                                  TTL=access 剩余
package auth

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// 用 //go:embed 把脚本编进二进制，避免运行期依赖文件系统布局。
// 脚本与本文件错位部署会导致 EVAL 报 NOSCRIPT 或行为漂移。
//
//go:embed lua/rotate.lua
var rotateScriptSource string

//go:embed lua/loginCreate.lua
var loginCreateScriptSource string

var (
	rotateScript      = redis.NewScript(rotateScriptSource)
	loginCreateScript = redis.NewScript(loginCreateScriptSource)
)

type LoginCreateParams struct {
	UID           uint
	SID           string
	RecordJSON    string
	RefreshTTLSec int
	ExpTS         int64
}

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

// RefreshRecord 对应 Redis 中 auth:refresh:{sid} 的 JSON 结构。
//
// 数据契约：`th` 是 sha256(rt_raw) 的 hex，原始 RT 永不落地——
// Redis 泄露场景下攻击者拿到 record 仍无法伪造合法 RT。
type RefreshRecord struct {
	TokenHash string `json:"th"`
	UID       uint   `json:"uid"`
	Exp       int64  `json:"exp"`
	UAHash    string `json:"ua,omitempty"`
	IP        string `json:"ip,omitempty"`
}

type SessionInfo struct {
	SID string
	Exp int64
}

// Sentinel errors —— API 契约：service 层通过 errors.Is 区分认证路径分支。
// 新增/变更 sentinel 即破坏接口语义，需同步审查所有 errors.Is 调用点。
var (
	ErrRotateExpired = errors.New("auth: refresh expired")
	ErrRotateReused  = errors.New("auth: refresh reused or corrupted")

	// ErrRefreshRecordCorrupted 标记 record JSON 解码失败。
	// 必须按 reused 路径处理：损坏可能是攻击痕迹，宽松处理等于绕过复用检测。
	ErrRefreshRecordCorrupted = errors.New("auth: refresh record corrupted")

	// ErrSessionNotFound sid 不存在。service 据此区分幂等登出 / 自然过期 / 复用检测。
	ErrSessionNotFound = errors.New("auth: session not found")
)

type Repository struct {
	rdb *redis.Client
}

func NewRepository(rdb *redis.Client) *Repository {
	return &Repository{rdb: rdb}
}

// Redis key 命名是跨进程契约——变更会让正在运行的旧实例与新实例看到不同的
// key 空间，等同强制下线所有用户。修改前必须做灰度迁移。
func refreshKey(sid string) string      { return "auth:refresh:" + sid }
func tombKey(sid string) string         { return "auth:refresh:tomb:" + sid }
func sessionsKey(uid uint) string       { return fmt.Sprintf("auth:user:%d:sessions", uid) }
func jtiBlacklistKey(jti string) string { return "auth:jti:blacklist:" + jti }

func (r *Repository) LoginCreate(ctx context.Context, p LoginCreateParams) error {
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

// RotateRefresh 错误码映射（必须与 rotate.lua 内的 return 码保持一致）：
//
//	0 → nil
//	1 → ErrRotateExpired
//	2 → ErrRotateReused（hash 不匹配 / 墓碑命中 / record JSON 损坏 三种合并）
//
// 数据契约：墓碑 value 必须是 uid 字符串。service 层在 tomb hit 路径下用它
// 触发 RevokeAllUserSessions，写其它格式会导致连坐失效。
func (r *Repository) RotateRefresh(ctx context.Context, p RotateParams) error {
	res, err := rotateScript.Run(ctx, r.rdb,
		[]string{
			refreshKey(p.OldSID), tombKey(p.OldSID),
			refreshKey(p.NewSID), sessionsKey(p.UID),
		},
		p.ExpectedHash, p.NewRecordJSON, p.RefreshTTLSec, p.TombTTLSec,
		p.NewSID, p.NewExpTS, p.OldSID,
		fmt.Sprintf("%d", p.UID), // 墓碑 value 契约：uid 十进制字符串。
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

// DeleteSession 返回 ErrSessionNotFound 表示 refresh key 已不存在；
// service 层据此实现登出幂等而不报错。
func (r *Repository) DeleteSession(ctx context.Context, uid uint, sid string) error {
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

func (r *Repository) GetRefreshRecord(ctx context.Context, sid string) (*RefreshRecord, error) {
	raw, err := r.rdb.Get(ctx, refreshKey(sid)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("redis get refresh record: %w", err)
	}
	var rec RefreshRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRefreshRecordCorrupted, err)
	}
	return &rec, nil
}

// GetTombUID 返回 ErrSessionNotFound 表示墓碑不存在——
// service 据此把"record 不存在 + 墓碑不存在"判定为自然过期而非复用。
func (r *Repository) GetTombUID(ctx context.Context, sid string) (uint, error) {
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

// RevokeAllUserSessions 用 Go pipeline 批删而非 Lua。
//
// 架构边界：sids 散落在多个 hash slot，Lua 在 Redis Cluster 下会触发
// CROSSSLOT 报错。即便后续切到 standalone 部署也不要回退到 Lua，
// 否则集群部署时会突然不可用。
//
// 一致性：最终一致——部分 sid DEL 失败不回滚已删项；调用方应告警但不阻塞业务。
func (r *Repository) RevokeAllUserSessions(ctx context.Context, uid uint) (int, error) {
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
		return len(sids), fmt.Errorf("redis pipeline revoke: %w", err)
	}
	return len(sids), nil
}

// ListUserSessions 副作用：顺手 ZREMRANGEBYSCORE 清掉过期 sid 索引。
// 索引清理失败不影响返回结果——sessions ZSet 无 key TTL，必须靠这条清理路径
// 控制无界增长。
func (r *Repository) ListUserSessions(ctx context.Context, uid uint) ([]SessionInfo, error) {
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
	// 维护动作，失败不影响本次结果。
	_ = r.rdb.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("(%d", now)).Err()
	return out, nil
}

// BlacklistJTI 不变量：ttl 必须 <= 对应 access token 的剩余寿命，
// 否则黑名单 key 会在 token 已自然失效后仍占用内存。
func (r *Repository) BlacklistJTI(ctx context.Context, jti string, ttl time.Duration) error {
	if strings.TrimSpace(jti) == "" {
		return fmt.Errorf("jti must not be empty")
	}
	if ttl <= 0 {
		return fmt.Errorf("blacklist ttl must be positive, got %v", ttl)
	}
	return r.rdb.Set(ctx, jtiBlacklistKey(jti), "1", ttl).Err()
}

func (r *Repository) IsJTIBlacklisted(ctx context.Context, jti string) (bool, error) {
	n, err := r.rdb.Exists(ctx, jtiBlacklistKey(jti)).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists jti blacklist: %w", err)
	}
	return n > 0, nil
}
