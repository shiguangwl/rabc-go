package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readDriver(path string) (string, error) {
	conf, err := readConfig(path)
	if err != nil {
		return "", err
	}
	return conf.GetString("data.db.user.driver"), nil
}

func TestNormalizeDialect(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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
			dsn:     "root:123456@tcp(127.0.0.1:3306)/user?charset=utf8mb4&parseTime=True&loc=Local",
			want:    "mysql://root:123456@127.0.0.1:3306/user?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name:    "mysql empty password drops colon",
			dialect: "mysql",
			dsn:     "root@tcp(127.0.0.1:3306)/db",
			want:    "mysql://root@127.0.0.1:3306/db",
		},
		{
			name:    "mysql password with at sign",
			dialect: "mysql",
			dsn:     "root:p@ssw0rd@tcp(127.0.0.1:3306)/db",
			want:    "mysql://root:p%40ssw0rd@127.0.0.1:3306/db",
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
		{
			name:    "mysql missing database",
			dialect: "mysql",
			dsn:     "root:123456@tcp(127.0.0.1:3306)/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

func TestActionSpecs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		action      string
		ensureDevDB bool
	}{
		{action: "diff", ensureDevDB: true},
		{action: "push", ensureDevDB: true},
		{action: "apply", ensureDevDB: false},
		{action: "status", ensureDevDB: false},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			t.Parallel()
			if actions[tt.action].ensureDevDB != tt.ensureDevDB {
				t.Fatalf("actions[%q].ensureDevDB = %v, want %v", tt.action, actions[tt.action].ensureDevDB, tt.ensureDevDB)
			}
		})
	}
}

func TestPostgresAdminDSN(t *testing.T) {
	t.Parallel()
	got, err := postgresAdminDSN("postgres://postgres:123456@127.0.0.1:5432/user?sslmode=disable")
	if err != nil {
		t.Fatalf("postgresAdminDSN() error = %v", err)
	}
	want := "postgres://postgres:123456@127.0.0.1:5432/postgres?sslmode=disable"
	if got != want {
		t.Fatalf("postgresAdminDSN() = %q, want %q", got, want)
	}
}

func TestBuildAtlasArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		action  string
		dialect string
		mName   string
		base    string
		want    []string
	}{
		{
			name:    "diff",
			action:  "diff",
			dialect: "mysql",
			mName:   "add_email",
			want:    []string{"migrate", "diff", "add_email", "--env", "local_mysql"},
		},
		{
			name:    "apply",
			action:  "apply",
			dialect: "postgres",
			want:    []string{"migrate", "apply", "--env", "local_postgres"},
		},
		{
			name:    "validate",
			action:  "validate",
			dialect: "mysql",
			want:    []string{"migrate", "validate", "--dir", "file://db/migrations/mysql"},
		},
		{
			name:    "hash",
			action:  "hash",
			dialect: "postgres",
			want:    []string{"migrate", "hash", "--dir", "file://db/migrations/postgres"},
		},
		{
			name:    "lint default latest 1",
			action:  "lint",
			dialect: "mysql",
			want:    []string{"migrate", "lint", "--env", "local_mysql", "--latest", "1"},
		},
		{
			name:    "lint with git base",
			action:  "lint",
			dialect: "mysql",
			base:    "origin/main",
			want:    []string{"migrate", "lint", "--env", "local_mysql", "--git-base", "origin/main"},
		},
		{
			name:    "push maps to schema apply auto-approve",
			action:  "push",
			dialect: "mysql",
			want:    []string{"schema", "apply", "--auto-approve", "--env", "local_mysql"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildAtlasArgs(tt.action, tt.dialect, tt.mName, tt.base)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildAtlasArgs()\n  got  = %q\n  want = %q", got, tt.want)
			}
		})
	}
}

func TestIsLocalDSN(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		dialect string
		dsn     string
		want    bool
		wantErr bool
	}{
		{name: "mysql 127.0.0.1", dialect: "mysql", dsn: "root:s@tcp(127.0.0.1:3306)/db", want: true},
		{name: "mysql localhost", dialect: "mysql", dsn: "root:s@tcp(localhost:3306)/db", want: true},
		{name: "mysql LOCALHOST upper", dialect: "mysql", dsn: "root:s@tcp(LOCALHOST:3306)/db", want: true},
		{name: "mysql unix socket", dialect: "mysql", dsn: "root:s@unix(/tmp/mysql.sock)/db", want: true},
		{name: "mysql ipv6 loopback", dialect: "mysql", dsn: "root:s@tcp([::1]:3306)/db", want: true},
		{name: "mysql lan ip is not local", dialect: "mysql", dsn: "root:s@tcp(10.0.0.5:3306)/db", want: false},
		{name: "mysql public domain is not local", dialect: "mysql", dsn: "root:s@tcp(db.prod.io:3306)/db", want: false},
		{name: "mysql 0.0.0.0 is not local", dialect: "mysql", dsn: "root:s@tcp(0.0.0.0:3306)/db", want: false},
		{name: "postgres 127.0.0.1", dialect: "postgres", dsn: "postgres://u:s@127.0.0.1:5432/db", want: true},
		{name: "postgres localhost", dialect: "postgres", dsn: "postgres://u:s@localhost/db", want: true},
		{name: "postgres LOCALHOST upper", dialect: "postgres", dsn: "postgres://u:s@LOCALHOST/db", want: true},
		{name: "postgres remote", dialect: "postgres", dsn: "postgres://u:s@db.prod.io:5432/db", want: false},
		{name: "unknown dialect errors", dialect: "sqlite", dsn: "file:foo.db", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := isLocalDSN(tt.dialect, tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatal("isLocalDSN() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("isLocalDSN() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("isLocalDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}
