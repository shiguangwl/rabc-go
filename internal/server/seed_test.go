package server

import "testing"

func TestIsLocalDSN(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		dsn    string
		want   bool
	}{
		{
			name:   "mysql tcp localhost",
			driver: "mysql",
			dsn:    "root:123456@tcp(127.0.0.1:3306)/user?charset=utf8mb4&parseTime=True&loc=Local",
			want:   true,
		},
		{
			name:   "mysql remote",
			driver: "mysql",
			dsn:    "root:123456@tcp(10.0.0.1:3306)/user",
			want:   false,
		},
		{
			name:   "postgres url localhost",
			driver: "postgres",
			dsn:    "postgres://postgres:123456@127.0.0.1:5432/user?sslmode=disable",
			want:   true,
		},
		{
			name:   "postgres keyword localhost",
			driver: "postgres",
			dsn:    "host=localhost user=postgres password=123456 dbname=user port=5432 sslmode=disable",
			want:   true,
		},
		{
			name:   "postgres remote",
			driver: "postgres",
			dsn:    "host=10.0.0.1 user=postgres password=123456 dbname=user port=5432 sslmode=disable",
			want:   false,
		},
		{
			name:   "unsupported driver",
			driver: "sqlite",
			dsn:    "file:test.db",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalDSN(tt.driver, tt.dsn); got != tt.want {
				t.Fatalf("isLocalDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTruncateTableSQL(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		table   string
		want    string
		wantErr bool
	}{
		{
			name:   "mysql",
			driver: "mysql",
			table:  "admin_users",
			want:   "TRUNCATE TABLE `admin_users`",
		},
		{
			name:   "postgres alias",
			driver: "postgresql",
			table:  "admin_users",
			want:   `TRUNCATE TABLE "admin_users" RESTART IDENTITY CASCADE`,
		},
		{
			name:    "unsupported",
			driver:  "sqlite",
			table:   "admin_users",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := truncateTableSQL(tt.driver, tt.table)
			if tt.wantErr {
				if err == nil {
					t.Fatal("truncateTableSQL() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("truncateTableSQL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("truncateTableSQL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeDBDriver(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		want   string
	}{
		{name: "mysql", driver: " MySQL ", want: "mysql"},
		{name: "postgres alias", driver: "PostgreSQL", want: "postgres"},
		{name: "unknown lower", driver: "SQLite", want: "sqlite"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeDBDriver(tt.driver); got != tt.want {
				t.Fatalf("normalizeDBDriver() = %q, want %q", got, tt.want)
			}
		})
	}
}
