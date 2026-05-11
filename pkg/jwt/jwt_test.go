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
