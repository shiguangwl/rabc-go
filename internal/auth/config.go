// Package auth 收敛认证子系统的跨层公共物，避免 pkg/jwt 基础库被业务概念污染。
package auth

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"rabc-go/pkg/log"
)

// AuthConfig 收敛认证子系统的运行参数，避免各层重复解析配置导致语义漂移。
//
// 字段语义见各项注释；上限保护与默认值由 LoadAuthConfig 在启动期一次性计算，
// 下游 service 不应再重新解析或覆盖。
type AuthConfig struct {
	// AccessTTL Access Token 寿命。短命无状态：仅 JWT 验签，不查 Redis。
	// 默认 30m；过短增加 refresh 频率打 Redis，过长扩大 token 泄露窗口。
	AccessTTL time.Duration

	// RefreshTTL Refresh Token 寿命。长命有状态：必须存 Redis，可吊销。
	// 默认 168h (7d)。
	RefreshTTL time.Duration

	// RotationTombTTL 轮换墓碑 TTL（独立配置，与 AccessTTL 解耦）。
	//
	// 墓碑用途：每次 refresh 轮换时把旧 sid 写一份 tombstone，覆盖"攻击者抢先
	// 轮换 → 受害者下次刷新墓碑命中 → 触发复用检测连坐"的窗口。
	//
	// 上限保护：实际生效值 = min(配置值, RefreshTTL)，避免无效占用 Redis 内存。
	RotationTombTTL time.Duration
}

const (
	defaultAccessTTL       = 30 * time.Minute
	defaultRefreshTTL      = 168 * time.Hour // 7d
	defaultRotationTombTTL = 30 * time.Minute
)

// LoadAuthConfig 启动期一次性读取并校验认证子系统参数。
//
// 设计约束：
//   - Duration 缺省或小于等于 0 时使用硬默认，避免得到不可用的 0 TTL。
//   - RotationTombTTL 大于 RefreshTTL 时截断到 RefreshTTL。
func LoadAuthConfig(conf *viper.Viper, logger *log.Logger) *AuthConfig {
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

	return &AuthConfig{
		AccessTTL:       accessTTL,
		RefreshTTL:      refreshTTL,
		RotationTombTTL: tombTTL,
	}
}

// durOrDefault 读 time.Duration，未配置或值为 0 时返回 fallback。
//
// viper.GetDuration 对缺失键返回 0，调用方需要显式兜底，避免 0 TTL 进入认证链路。
func durOrDefault(conf *viper.Viper, key string, fallback time.Duration) time.Duration {
	d := conf.GetDuration(key)
	if d <= 0 {
		return fallback
	}
	return d
}
