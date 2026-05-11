package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// readDriver 是 readConfig 在 driver 读取路径上的薄包装，
// 仅服务于 TestReadDriver* 验证「文件 + APP_DATA_DB_USER_DRIVER 环境变量覆盖」语义。
// 留在 _test.go 而不放进生产代码：生产路径直接走 readConfig + GetString。
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
			dsn:     "root:123456@tcp(127.0.0.1:3380)/user?charset=utf8mb4&parseTime=True&loc=Local",
			want:    "mysql://root:123456@127.0.0.1:3380/user?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name:    "mysql empty password drops colon",
			dialect: "mysql",
			dsn:     "root@tcp(127.0.0.1:3306)/db",
			want:    "mysql://root@127.0.0.1:3306/db",
		},
		{
			// 这是 strings.Cut → mysql.ParseDSN 切换的核心动机：
			// 手写按 ":" / "@" 切片对密码内含 "@" 会切错。
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
			want:    []string{"migrate", "validate", "--env", "local_mysql"},
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
		// 回归用例：DSN 解析后保留 host 原始大小写，isLocalHost 必须做大小写归一化。
		{name: "mysql LOCALHOST upper", dialect: "mysql", dsn: "root:s@tcp(LOCALHOST:3306)/db", want: true},
		// 回归用例：unix socket 显式判定，避免 net.SplitHostPort 报错被吞而"巧合通过"。
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
