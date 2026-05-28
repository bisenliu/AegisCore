package config

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadPostgresSharedConfig(t *testing.T) {
	cfg := loadConfigFromYAML(t, `
database:
  postgres:
    host: 127.0.0.1
    username: aegiscore
    password: secret
    user_db_name: aegiscore_user
    pay_db_name: aegiscore_pay
    common_db_name: aegiscore_common
`)

	pg := cfg.Database.Postgres
	if pg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want 127.0.0.1", pg.Host)
	}
	if pg.Port != 5432 {
		t.Fatalf("Port = %d, want 5432", pg.Port)
	}
	if pg.Driver != "pgx" {
		t.Fatalf("Driver = %q, want pgx", pg.Driver)
	}
	if pg.SSLMode != "disable" {
		t.Fatalf("SSLMode = %q, want disable", pg.SSLMode)
	}
	if pg.MaxOpenConns != 25 {
		t.Fatalf("MaxOpenConns = %d, want 25", pg.MaxOpenConns)
	}
	if pg.MaxIdleConns != 5 {
		t.Fatalf("MaxIdleConns = %d, want 5", pg.MaxIdleConns)
	}
	if pg.ConnMaxLifetime != 30*time.Minute {
		t.Fatalf("ConnMaxLifetime = %s, want 30m", pg.ConnMaxLifetime)
	}
	if pg.ConnMaxIdleTime != 10*time.Minute {
		t.Fatalf("ConnMaxIdleTime = %s, want 10m", pg.ConnMaxIdleTime)
	}
	if pg.PingTimeout != 5*time.Second {
		t.Fatalf("PingTimeout = %s, want 5s", pg.PingTimeout)
	}
	if pg.PayDBName != "aegiscore_pay" {
		t.Fatalf("PayDBName = %q, want aegiscore_pay", pg.PayDBName)
	}
}

func TestLoadRequiresPostgresFields(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "host",
			yaml: `
database:
  postgres:
    username: aegiscore
    user_db_name: aegiscore_user
    common_db_name: aegiscore_common
`,
			wantErr: "database.postgres.host is required",
		},
		{
			name: "port",
			yaml: `
database:
  postgres:
    host: 127.0.0.1
    port: -1
    username: aegiscore
    user_db_name: aegiscore_user
    common_db_name: aegiscore_common
`,
			wantErr: "database.postgres.port must be between 1 and 65535",
		},
		{
			name: "username",
			yaml: `
database:
  postgres:
    host: 127.0.0.1
    user_db_name: aegiscore_user
    common_db_name: aegiscore_common
`,
			wantErr: "database.postgres.username is required",
		},
		{
			name: "user db name",
			yaml: `
database:
  postgres:
    host: 127.0.0.1
    username: aegiscore
    common_db_name: aegiscore_common
`,
			wantErr: "database.postgres.user_db_name is required",
		},
		{
			name: "common db name",
			yaml: `
database:
  postgres:
    host: 127.0.0.1
    username: aegiscore
    user_db_name: aegiscore_user
`,
			wantErr: "database.postgres.common_db_name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeTempConfig(t, tt.yaml))
			if err == nil {
				t.Fatal("Load returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPostgresNamedDatabaseDSNs(t *testing.T) {
	cfg := loadConfigFromYAML(t, `
database:
  postgres:
    host: db.example.internal
    port: 15432
    username: user@example.com
    password: p@ss/w:rd
    user_db_name: user_db
    pay_db_name: pay_db
    common_db_name: common_db
`)

	tests := []struct {
		name       string
		wantDBName string
	}{
		{name: "user_db", wantDBName: "user_db"},
		{name: "common_db", wantDBName: "common_db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, ok := cfg.Postgres(tt.name)
			if !ok {
				t.Fatalf("Postgres(%q) ok = false", tt.name)
			}
			parsed, err := url.Parse(db.DSN)
			if err != nil {
				t.Fatalf("Parse DSN: %v", err)
			}
			if parsed.Scheme != "postgres" {
				t.Fatalf("Scheme = %q, want postgres", parsed.Scheme)
			}
			if parsed.Host != "db.example.internal:15432" {
				t.Fatalf("Host = %q, want db.example.internal:15432", parsed.Host)
			}
			if got := strings.TrimPrefix(parsed.Path, "/"); got != tt.wantDBName {
				t.Fatalf("database name = %q, want %q", got, tt.wantDBName)
			}
			if parsed.User.Username() != "user@example.com" {
				t.Fatalf("username = %q, want user@example.com", parsed.User.Username())
			}
			password, ok := parsed.User.Password()
			if !ok || password != "p@ss/w:rd" {
				t.Fatalf("password = %q, %v; want p@ss/w:rd, true", password, ok)
			}
			if parsed.Query().Get("sslmode") != "disable" {
				t.Fatalf("sslmode = %q, want disable", parsed.Query().Get("sslmode"))
			}
		})
	}

	if _, ok := cfg.Postgres("pay_db"); !ok {
		t.Fatal("Postgres(pay_db) ok = false")
	}
	if _, ok := cfg.Postgres("missing_db"); ok {
		t.Fatal("Postgres(missing_db) ok = true")
	}
}

func loadConfigFromYAML(t *testing.T, content string) *Config {
	t.Helper()
	cfg, err := Load(writeTempConfig(t, content))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
