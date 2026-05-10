package config

import (
	"reflect"
	"testing"
)

func TestEnvNameForKey(t *testing.T) {
	if got, want := envNameForKey("security.jwt.key"), "APP_SECURITY_JWT_KEY"; got != want {
		t.Fatalf("envNameForKey() = %q, want %q", got, want)
	}
}

func TestMissingEnvNames(t *testing.T) {
	got := missingEnvNames([]string{"security.jwt.key", "data.db.user.dsn"})
	want := []string{"APP_SECURITY_JWT_KEY", "APP_DATA_DB_USER_DSN"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missingEnvNames() = %#v, want %#v", got, want)
	}
}
