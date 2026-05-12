package jwt

import (
	"errors"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

func TestParseTokenRejectsUnexpectedAlg(t *testing.T) {
	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret")
	j := NewJwt(conf)

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS384, MyCustomClaims{
		UserID: 1,
		RegisteredClaims: jwtv5.RegisteredClaims{
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	raw, err := token.SignedString(j.key)
	if err != nil {
		t.Fatal(err)
	}

	_, err = j.ParseToken(raw)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseToken() error = %v, want ErrInvalidToken", err)
	}
}

func TestParseTokenLegacyUserID(t *testing.T) {
	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret")
	j := NewJwt(conf)

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, jwtv5.MapClaims{
		"userId": float64(7),
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	raw, err := token.SignedString(j.key)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := j.ParseToken(raw)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != 7 {
		t.Fatalf("UserID = %d, want 7", claims.UserID)
	}
}

func TestParseTokenLegacyUserIdField(t *testing.T) {
	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret")
	j := NewJwt(conf)

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, jwtv5.MapClaims{
		"UserId": float64(9),
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	raw, err := token.SignedString(j.key)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := j.ParseToken(raw)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != 9 {
		t.Fatalf("UserID = %d, want 9", claims.UserID)
	}
}

func TestParseTokenRejectsMissingUserID(t *testing.T) {
	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret")
	j := NewJwt(conf)

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, jwtv5.RegisteredClaims{
		ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
	})
	raw, err := token.SignedString(j.key)
	if err != nil {
		t.Fatal(err)
	}

	_, err = j.ParseToken(raw)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseToken() error = %v, want ErrInvalidToken", err)
	}
}

func TestParseTokenRejectsSubjectMismatch(t *testing.T) {
	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret")
	j := NewJwt(conf)

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, MyCustomClaims{
		UserID: 3,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Subject:   "4",
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	raw, err := token.SignedString(j.key)
	if err != nil {
		t.Fatal(err)
	}

	_, err = j.ParseToken(raw)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseToken() error = %v, want ErrInvalidToken", err)
	}
}

func TestGenTokenExtrasRoundTrip(t *testing.T) {
	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret")
	j := NewJwt(conf)

	raw, err := j.GenToken(42, time.Now().Add(time.Hour), map[string]any{
		"sid": "abc123",
		"deg": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := j.ParseToken(raw)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", claims.UserID)
	}
	sid, ok := claims.ExtString("sid")
	if !ok || sid != "abc123" {
		t.Fatalf("ExtString(sid) = (%q, %v), want (\"abc123\", true)", sid, ok)
	}
	deg, ok := claims.ExtBool("deg")
	if !ok || deg != true {
		t.Fatalf("ExtBool(deg) = (%v, %v), want (true, true)", deg, ok)
	}
	if claims.ID == "" {
		t.Fatalf("RegisteredClaims.ID (jti) should be auto-generated, got empty")
	}
}

// 无 ext 字段的 token 必须保持兼容，避免部署期间强制用户重登。
func TestParseTokenLegacyNoExtFieldCompatible(t *testing.T) {
	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret")
	j := NewJwt(conf)

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, jwtv5.MapClaims{
		"uid": float64(11),
		"sub": "11",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	raw, err := token.SignedString(j.key)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := j.ParseToken(raw)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != 11 {
		t.Fatalf("UserID = %d, want 11", claims.UserID)
	}
	if claims.Extras != nil {
		t.Fatalf("Extras should be nil when ext field absent, got %v", claims.Extras)
	}
	if _, ok := claims.Ext("deg"); ok {
		t.Fatalf("Ext on nil Extras should return ok=false")
	}
	if _, ok := claims.ExtBool("deg"); ok {
		t.Fatalf("ExtBool on nil Extras should return ok=false")
	}
}

// 显式 nil 必须视为字段缺失，避免未设置的扩展字段触发类型错误。
func TestExtBoolExplicitNilTreatedAsMissing(t *testing.T) {
	claims := &MyCustomClaims{Extras: map[string]any{"deg": nil}}

	v, ok := claims.ExtBool("deg")
	if v != false || ok != false {
		t.Fatalf("ExtBool with explicit nil = (%v, %v), want (false, false)", v, ok)
	}

	rawV, rawOK := claims.Ext("deg")
	if rawV != nil {
		t.Fatalf("Ext(deg) value = %v, want nil", rawV)
	}
	if !rawOK {
		t.Fatalf("Ext(deg) ok = false, want true (key exists in map)")
	}
}

// 类型错误（Extras["deg"]="true" string）必须返 (false, false) 但 Ext 返存在。
// middleware 用 Ext 二次检查"字段存在性"决定是否 fail-loud 500。
func TestExtBoolTypeMismatchExposedToMiddleware(t *testing.T) {
	claims := &MyCustomClaims{Extras: map[string]any{"deg": "true"}}

	v, ok := claims.ExtBool("deg")
	if v != false || ok != false {
		t.Fatalf("ExtBool with string \"true\" = (%v, %v), want (false, false)", v, ok)
	}
	// middleware 用 Ext 区分"字段存在但类型错"——必须返 ok=true
	_, rawOK := claims.Ext("deg")
	if !rawOK {
		t.Fatalf("Ext(deg) ok = false, want true (field present but wrong type)")
	}
}

// ExtString 同样处理显式 nil 与类型错误。
func TestExtStringHandlesNilAndTypeMismatch(t *testing.T) {
	tests := []struct {
		name    string
		extras  map[string]any
		wantVal string
		wantOK  bool
		wantExt bool
	}{
		{"missing", nil, "", false, false},
		{"explicit_nil", map[string]any{"sid": nil}, "", false, true},
		{"type_int", map[string]any{"sid": 123}, "", false, true},
		{"valid_string", map[string]any{"sid": "hello"}, "hello", true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims := &MyCustomClaims{Extras: tc.extras}
			v, ok := claims.ExtString("sid")
			if v != tc.wantVal || ok != tc.wantOK {
				t.Fatalf("ExtString = (%q, %v), want (%q, %v)", v, ok, tc.wantVal, tc.wantOK)
			}
			if _, extOK := claims.Ext("sid"); extOK != tc.wantExt {
				t.Fatalf("Ext ok = %v, want %v", extOK, tc.wantExt)
			}
		})
	}
}

func TestParseTokenLeewayAllows25sAfterExpiry(t *testing.T) {
	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret")
	j := NewJwt(conf)

	expiredAt := time.Now().Add(-25 * time.Second)
	raw, err := j.GenToken(7, expiredAt, nil)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := j.ParseToken(raw)
	if err != nil {
		t.Fatalf("ParseToken within leeway should succeed, got error: %v", err)
	}
	if claims.UserID != 7 {
		t.Fatalf("UserID = %d, want 7", claims.UserID)
	}
}

func TestParseTokenLeewayRejects35sAfterExpiry(t *testing.T) {
	conf := viper.New()
	conf.Set("security.jwt.key", "test-secret")
	j := NewJwt(conf)

	expiredAt := time.Now().Add(-35 * time.Second)
	raw, err := j.GenToken(7, expiredAt, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = j.ParseToken(raw)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseToken beyond leeway should fail with ErrInvalidToken, got: %v", err)
	}
}
