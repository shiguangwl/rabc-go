package auth

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"rabc-go/pkg/log"
)

// Config 收敛认证子系统的运行参数。
//
// 不变量：所有字段在启动期由 LoadConfig 一次性计算并冻结，下游 service 不得
// 重新解析或覆盖，否则会与已发出的 token 寿命产生不一致。
type Config struct {
	// AccessTTL Access Token 寿命。无状态：仅 JWT 验签，不查 Redis。
	// 取值权衡：过短增加 refresh 频率打 Redis；过长扩大 token 泄露窗口。
	AccessTTL time.Duration

	// RefreshTTL Refresh Token 寿命。有状态：存 Redis，可吊销。
	RefreshTTL time.Duration

	// RotationTombTTL 轮换墓碑 TTL。
	//
	// 墓碑用途：每次 refresh 轮换时把旧 sid 写为 tombstone，覆盖"攻击者抢先
	// 轮换 → 受害者下次刷新命中墓碑 → 触发复用检测连坐"的窗口。
	//
	// 不变量：生效值上限被截断到 RefreshTTL——超过 RT 寿命的墓碑无安全收益。
	RotationTombTTL time.Duration
}

const (
	defaultAccessTTL       = 30 * time.Minute
	defaultRefreshTTL      = 168 * time.Hour // 7d
	defaultRotationTombTTL = 30 * time.Minute
)

// LoadConfig 启动期读取并校验认证参数。
//
// 兜底契约：
//   - 任一 TTL 缺省或 <=0 时回退到硬默认；0 TTL 进入认证链路会立即让所有 token 失效。
//   - RotationTombTTL 被截断到 <= RefreshTTL。
func LoadConfig(conf *viper.Viper, logger *log.Logger) *Config {
	accessTTL := durOrDefault(conf, "security.jwt.access_ttl", defaultAccessTTL)
	refreshTTL := durOrDefault(conf, "security.jwt.refresh_ttl", defaultRefreshTTL)
	tombTTL := durOrDefault(conf, "security.auth.rotation_tomb_ttl", defaultRotationTombTTL)

	// 墓碑超过 refresh 生命周期没有安全收益，只会增加存储占用。
	if tombTTL > refreshTTL {
		logger.Warn("rotation_tomb_ttl > refresh_ttl, truncated to refresh_ttl",
			zap.Duration("configured", tombTTL),
			zap.Duration("refresh_ttl", refreshTTL))
		tombTTL = refreshTTL
	}

	return &Config{
		AccessTTL:       accessTTL,
		RefreshTTL:      refreshTTL,
		RotationTombTTL: tombTTL,
	}
}

// durOrDefault 读 time.Duration，<=0 时回退 fallback。
//
// 必须显式兜底：viper.GetDuration 对缺失键返回 0，直接用会导致 0 TTL 令牌
// 在认证链路被立刻判过期。
func durOrDefault(conf *viper.Viper, key string, fallback time.Duration) time.Duration {
	d := conf.GetDuration(key)
	if d <= 0 {
		return fallback
	}
	return d
}
