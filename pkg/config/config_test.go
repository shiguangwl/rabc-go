package config

import (
	"os"
	"path/filepath"
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

func TestMissingConfigHints(t *testing.T) {
	got := missingConfigHints([]string{"security.jwt.key", "data.db.user.dsn"})
	want := []string{
		"security.jwt.key or APP_SECURITY_JWT_KEY",
		"data.db.user.dsn or APP_DATA_DB_USER_DSN",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missingConfigHints() = %#v, want %#v", got, want)
	}
}

func TestNewConfigReadsProdYAMLWhenEnvAbsent(t *testing.T) {
	path := writeConfig(t, `
env: prod
security:
  jwt:
    key: yaml-secret
data:
  db:
    user:
      dsn: yaml-dsn
`)

	conf := NewConfig(path)

	if got := conf.GetString("security.jwt.key"); got != "yaml-secret" {
		t.Fatalf("security.jwt.key = %q, want yaml-secret", got)
	}
	if got := conf.GetString("data.db.user.dsn"); got != "yaml-dsn" {
		t.Fatalf("data.db.user.dsn = %q, want yaml-dsn", got)
	}
}

func TestNewConfigEnvOverridesProdYAML(t *testing.T) {
	t.Setenv("APP_SECURITY_JWT_KEY", "env-secret")
	t.Setenv("APP_DATA_DB_USER_DSN", "env-dsn")
	path := writeConfig(t, `
env: prod
security:
  jwt:
    key: yaml-secret
data:
  db:
    user:
      dsn: yaml-dsn
`)

	conf := NewConfig(path)

	if got := conf.GetString("security.jwt.key"); got != "env-secret" {
		t.Fatalf("security.jwt.key = %q, want env-secret", got)
	}
	if got := conf.GetString("data.db.user.dsn"); got != "env-dsn" {
		t.Fatalf("data.db.user.dsn = %q, want env-dsn", got)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
