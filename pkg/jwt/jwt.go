package jwt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

// Sentinel 错误，调用方可通过 errors.Is 判定。
// 解析过程中的具体原因（过期 / 签名错 / 格式错）通过 %w 链保留，
// 上层若需要细分判定可继续用 errors.Is(err, jwt.ErrTokenExpired) 等。
var (
	ErrTokenEmpty   = errors.New("jwt: token is empty")
	ErrInvalidToken = errors.New("jwt: invalid token")
)

type JWT struct {
	key []byte
}

type MyCustomClaims struct {
	UserID uint `json:"uid"`
	jwt.RegisteredClaims
}

func (c *MyCustomClaims) UnmarshalJSON(data []byte) error {
	aux := struct {
		UserID       *uint `json:"uid"`
		LegacyUserID *uint `json:"userId"`
		LegacyUserId *uint `json:"UserId"`
		jwt.RegisteredClaims
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.RegisteredClaims = aux.RegisteredClaims
	switch {
	case aux.UserID != nil:
		c.UserID = *aux.UserID
	case aux.LegacyUserID != nil:
		c.UserID = *aux.LegacyUserID
	case aux.LegacyUserId != nil:
		c.UserID = *aux.LegacyUserId
	}
	return nil
}

func NewJwt(conf *viper.Viper) *JWT {
	key := conf.GetString("security.jwt.key")
	if len(key) == 0 {
		panic("jwt key must not be empty")
	}
	return &JWT{key: []byte(key)}
}

func (j *JWT) GenToken(userId uint, expiresAt time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, MyCustomClaims{
		UserID: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userId), 10),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
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
	token, err := jwt.ParseWithClaims(tokenString, &MyCustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
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
	return claims, nil
}
