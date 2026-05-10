package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeDialect(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		want    string
		wantErr bool
	}{
		{name: "empty defaults mysql", driver: "", want: "mysql"},
		{name: "mysql", driver: "mysql", want: "mysql"},
		{name: "postgres", driver: "postgres", want: "postgres"},
		{name: "postgresql alias", driver: "postgresql", want: "postgres"},
		{name: "trim and case", driver: " PostgreSQL ", want: "postgres"},
		{name: "unsupported", driver: "sqlite", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeDialect(tt.driver)
			if tt.wantErr {
				if err == nil {
					t.Fatal("normalizeDialect() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeDialect() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeDialect() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadDriver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("data:\n  db:\n    user:\n      driver: postgres\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readDriver(path)
	if err != nil {
		t.Fatalf("readDriver() error = %v", err)
	}
	if got != "postgres" {
		t.Fatalf("readDriver() = %q, want postgres", got)
	}
}

func TestReadDriverEnvOverride(t *testing.T) {
	t.Setenv("APP_DATA_DB_USER_DRIVER", "postgres")
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("data:\n  db:\n    user:\n      driver: mysql\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readDriver(path)
	if err != nil {
		t.Fatalf("readDriver() error = %v", err)
	}
	if got != "postgres" {
		t.Fatalf("readDriver() = %q, want postgres from env", got)
	}
}

func TestAtlasURL(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
		dsn     string
		want    string
		wantErr bool
	}{
		{
			name:    "mysql tcp",
			dialect: "mysql",
			dsn:     "root:123456@tcp(127.0.0.1:3380)/user?charset=utf8mb4&parseTime=True&loc=Local",
			want:    "mysql://root:123456@127.0.0.1:3380/user?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name:    "postgres passthrough",
			dialect: "postgres",
			dsn:     "postgres://postgres:123456@127.0.0.1:5432/user?sslmode=disable",
			want:    "postgres://postgres:123456@127.0.0.1:5432/user?sslmode=disable",
		},
		{
			name:    "mysql unix unsupported",
			dialect: "mysql",
			dsn:     "root:123456@unix(/tmp/mysql.sock)/user",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := atlasURL(tt.dialect, tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatal("atlasURL() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("atlasURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("atlasURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
