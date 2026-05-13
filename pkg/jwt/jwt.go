package jwt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// Sentinel 错误，调用方可通过 errors.Is 判定。
// 解析过程中的具体原因（过期 / 签名错 / 格式错）通过 %w 链保留，
// 上层若需要细分判定可继续用 errors.Is(err, jwt.ErrTokenExpired) 等。
var (
	ErrTokenEmpty   = errors.New("jwt: token is empty")
	ErrInvalidToken = errors.New("jwt: invalid token")
)

// parseLeeway 是 ParseToken 允许的时钟偏差容忍度。
//
// 多实例部署允许少量 NTP 漂移，但容忍窗口必须明显小于 access token TTL。
const parseLeeway = 30 * time.Second

type JWT struct {
	key []byte
}

// MyCustomClaims 自定义 JWT 业务字段。
//
// UserID 是中间件必读字段；Extras 只承载业务扩展字段，pkg/jwt 不解释其语义。
type MyCustomClaims struct {
	UserID uint           `json:"uid"`
	Extras map[string]any `json:"ext,omitempty"` // 业务扩展字段；缺失或空时不序列化。
	jwt.RegisteredClaims
}

func (c *MyCustomClaims) UnmarshalJSON(data []byte) error {
	aux := struct {
		UserID            *uint          `json:"uid"`
		LegacyUserID      *uint          `json:"userID"`
		LegacyUpperUserID *uint          `json:"UserId"`
		Extras            map[string]any `json:"ext,omitempty"`
		jwt.RegisteredClaims
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.RegisteredClaims = aux.RegisteredClaims
	c.Extras = aux.Extras
	switch {
	case aux.UserID != nil:
		c.UserID = *aux.UserID
	case aux.LegacyUserID != nil:
		c.UserID = *aux.LegacyUserID
	case aux.LegacyUpperUserID != nil:
		c.UserID = *aux.LegacyUpperUserID
	}
	return nil
}

// Ext 取 Extras 中指定 key 的原始值与存在性。
// 与 map[k] 二值取值不同：显式 nil 值（m[k]=nil）也返回 (nil, true)。
// 调用方需要区分"字段缺失" vs "字段显式 nil" 时使用本方法 + nil 比较。
func (c *MyCustomClaims) Ext(k string) (any, bool) {
	if c.Extras == nil {
		return nil, false
	}
	v, ok := c.Extras[k]
	return v, ok
}

// ExtBool 取 Extras 中的 bool 字段，返回值 + 类型 OK 标记。
//
// 显式 nil 视为字段缺失，避免把未设置的扩展字段误判为类型错误。
//
// 中间件区分语义：
//   - (val, true)            字段存在且类型 bool 正确
//   - (false, false) + Ext 返 (_, false)  字段缺失，正常路径
//   - (false, false) + Ext 返 (_, true)   字段存在但类型错（fail-loud → 500 + audit）
func (c *MyCustomClaims) ExtBool(k string) (bool, bool) {
	v, exists := c.Extras[k]
	if !exists || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// ExtString 同 ExtBool 模式，处理 string 字段。
// 显式 nil 同样视为字段缺失。
func (c *MyCustomClaims) ExtString(k string) (string, bool) {
	v, exists := c.Extras[k]
	if !exists || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func NewJwt(conf *viper.Viper) *JWT {
	key := conf.GetString("security.jwt.key")
	if len(key) == 0 {
		panic("jwt key must not be empty")
	}
	return &JWT{key: []byte(key)}
}

// GenToken 签发 access token。
//
// extras 是业务字段 carry（如 {"sid": "...", "deg": true}），pkg/jwt 不解释
// 语义，直接 marshal 进 ext 顶层字段；下游 ExtBool/ExtString 取值。
// 传 nil 时不序列化 ext 字段（omitempty）。
//
// jti（RegisteredClaims.ID）由 uuid.NewString() 自动生成，供吊销和审计使用。
func (j *JWT) GenToken(userID uint, expiresAt time.Time, extras map[string]any) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, MyCustomClaims{
		UserID: userID,
		Extras: extras,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userID), 10),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	})

	tokenString, err := token.SignedString(j.key)
	if err != nil {
		return "", fmt.Errorf("jwt: sign token: %w", err)
	}
	return tokenString, nil
}

func (j *JWT) ParseToken(tokenString string) (*MyCustomClaims, error) {
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	if strings.TrimSpace(tokenString) == "" {
		return nil, ErrTokenEmpty
	}
	token, err := jwt.ParseWithClaims(tokenString, &MyCustomClaims{}, func(_ *jwt.Token) (interface{}, error) {
		return j.key, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithLeeway(parseLeeway),
	)
	if err != nil {
		// 双 %w 链：errors.Is 可同时命中业务 sentinel ErrInvalidToken 与
		// 具体原因（如 jwtv5.ErrTokenExpired）。这两层是"类别 + 原因"的关系，
		// 不是互斥分支，下游需要哪一层就 errors.Is 哪一层。
		// 不用 errors.Join：Join 的 Error() 会拼接换行，污染单行日志聚合。
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	claims, ok := token.Claims.(*MyCustomClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.UserID == 0 {
		return nil, fmt.Errorf("%w: uid is required", ErrInvalidToken)
	}
	if claims.Subject != "" && claims.Subject != strconv.FormatUint(uint64(claims.UserID), 10) {
		return nil, fmt.Errorf("%w: subject does not match uid", ErrInvalidToken)
	}
	return claims, nil
}
