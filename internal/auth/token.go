package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// RT 二进制布局是跨进程契约：32B raw（16B sid || 16B nonce）→
// base64.RawURLEncoding 输出固定 43 字符。
//
// 安全约束：sid 与 nonce 各保留 128 bit 不可预测性。缩短任一段都会让 sid
// 枚举或 hash(rt) 输入碰撞成为可行攻击。
//
// 调整长度会让所有在用 RT 失效——前端需配合发版同步清 token。
const (
	rtSIDLen  = 16
	rtRandLen = 16
	rtRawLen  = rtSIDLen + rtRandLen // 32

	RTEncodedLen = 43
)

// ErrRefreshFormat —— API 契约：service 据此把请求映射为 401，不暴露格式细节。
var ErrRefreshFormat = errors.New("auth: refresh token format invalid")

// ParseRT 严格校验：base64 解码后长度必须严格等于 rtRawLen。
// 放宽长度校验等于把 RT 当变长字符串处理，会破坏 sid/nonce 二段切分契约。
func ParseRT(raw string) (sid, rnd string, err error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", "", fmt.Errorf("%w: base64 decode: %w", ErrRefreshFormat, err)
	}
	if len(decoded) != rtRawLen {
		return "", "", fmt.Errorf("%w: expected %dB, got %dB",
			ErrRefreshFormat, rtRawLen, len(decoded))
	}
	sid = hex.EncodeToString(decoded[:rtSIDLen])
	rnd = hex.EncodeToString(decoded[rtSIDLen:])
	return sid, rnd, nil
}

// GenRT 必须使用 crypto/rand：math/rand 的可预测性会让 sid 与 nonce 同时
// 落入攻击者可枚举范围。sid 用 hex 作 Redis key 后缀，避免 base64 特殊字符。
func GenRT() (raw, sid string, err error) {
	buf := make([]byte, rtRawLen)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("crypto/rand read: %w", err)
	}
	sid = hex.EncodeToString(buf[:rtSIDLen])
	return base64.RawURLEncoding.EncodeToString(buf), sid, nil
}

// HashRT 是 record.th 与 RotateRefresh.ExpectedHash 的唯一来源。
// 两边必须用同一函数，任何一侧改算法都会让所有在用 RT 立即被判复用。
func HashRT(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
