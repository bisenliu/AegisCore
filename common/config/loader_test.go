package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadExplicitConfig(t *testing.T) {
	cfg := loadConfigFromYAML(t, explicitConfigYAML())

	if cfg.App.Name != "aegiscore-test" {
		t.Fatalf("App.Name = %q, want aegiscore-test", cfg.App.Name)
	}
	if cfg.HTTP.Port != 18080 {
		t.Fatalf("HTTP.Port = %d, want 18080", cfg.HTTP.Port)
	}
	if cfg.Redis.DB != 2 {
		t.Fatalf("Redis.DB = %d, want 2", cfg.Redis.DB)
	}

	pg := cfg.Database.Postgres
	if pg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want 127.0.0.1", pg.Host)
	}
	if pg.Port != 15432 {
		t.Fatalf("Port = %d, want 15432", pg.Port)
	}
	if pg.Driver != "pgx" {
		t.Fatalf("Driver = %q, want pgx", pg.Driver)
	}
	if pg.SSLMode != "disable" {
		t.Fatalf("SSLMode = %q, want disable", pg.SSLMode)
	}
	if pg.MaxOpenConns != 20 {
		t.Fatalf("MaxOpenConns = %d, want 20", pg.MaxOpenConns)
	}
	if pg.MaxIdleConns != 4 {
		t.Fatalf("MaxIdleConns = %d, want 4", pg.MaxIdleConns)
	}
	if pg.ConnMaxLifetime != 45*time.Minute {
		t.Fatalf("ConnMaxLifetime = %s, want 45m", pg.ConnMaxLifetime)
	}
	if pg.ConnMaxIdleTime != 12*time.Minute {
		t.Fatalf("ConnMaxIdleTime = %s, want 12m", pg.ConnMaxIdleTime)
	}
	if pg.PingTimeout != 7*time.Second {
		t.Fatalf("PingTimeout = %s, want 7s", pg.PingTimeout)
	}
	if pg.PayDBName != "aegiscore_pay" {
		t.Fatalf("PayDBName = %q, want aegiscore_pay", pg.PayDBName)
	}
}

func TestLoadDoesNotValidateMissingPrimaryConfigFields(t *testing.T) {
	cfg := loadConfigFromYAML(t, `app:
  environment: test

http:
  port: 0

log: {}

redis:
  db: 0

database:
  postgres:
    port: 0
`)

	if cfg.App.Name != "" {
		t.Fatalf("App.Name = %q, want empty", cfg.App.Name)
	}
	if cfg.HTTP.Host != "" {
		t.Fatalf("HTTP.Host = %q, want empty", cfg.HTTP.Host)
	}
	if cfg.Redis.Addr != "" {
		t.Fatalf("Redis.Addr = %q, want empty", cfg.Redis.Addr)
	}
	if cfg.Database.Postgres.UserDBName != "" {
		t.Fatalf("UserDBName = %q, want empty", cfg.Database.Postgres.UserDBName)
	}
	if _, ok := cfg.Postgres("user_db"); ok {
		t.Fatal("Postgres(user_db) ok = true")
	}
}

func TestLoadDoesNotValidateInvalidBasicValues(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSection(`http:
  host: 127.0.0.1
  port: 70000
  read_timeout: 0s
  write_timeout: 0s
  idle_timeout: 0s
  shutdown_timeout: 0s`))
	if cfg.HTTP.Port != 70000 {
		t.Fatalf("HTTP.Port = %d, want 70000", cfg.HTTP.Port)
	}
	if cfg.HTTP.ReadTimeout != 0 {
		t.Fatalf("HTTP.ReadTimeout = %s, want 0s", cfg.HTTP.ReadTimeout)
	}

	cfg = loadConfigFromYAML(t, configYAMLWithSection(`redis:
  addr: 127.0.0.1:6379
  db: -1
  dial_timeout: 0s
  read_timeout: 0s
  write_timeout: 0s`))
	if cfg.Redis.DB != -1 {
		t.Fatalf("Redis.DB = %d, want -1", cfg.Redis.DB)
	}
	if cfg.Redis.DialTimeout != 0 {
		t.Fatalf("Redis.DialTimeout = %s, want 0s", cfg.Redis.DialTimeout)
	}

	cfg = loadConfigFromYAML(t, configYAMLWithSection(`database:
  postgres:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    password: secret
    user_db_name: aegiscore_user
    pay_db_name: aegiscore_pay
    common_db_name: aegiscore_common
    driver: pgx
    sslmode: disable
    max_open_conns: 0
    max_idle_conns: 0
    conn_max_lifetime: 0s
    conn_max_idle_time: 0s
    ping_timeout: 0s`))
	if cfg.Database.Postgres.MaxOpenConns != 0 {
		t.Fatalf("MaxOpenConns = %d, want 0", cfg.Database.Postgres.MaxOpenConns)
	}
	if cfg.Database.Postgres.PingTimeout != 0 {
		t.Fatalf("PingTimeout = %s, want 0s", cfg.Database.Postgres.PingTimeout)
	}
}

func TestLoadEnvironmentOverride(t *testing.T) {
	t.Setenv("AEGISCORE_HTTP_PORT", "19090")
	t.Setenv("AEGISCORE_DATABASE_POSTGRES_PASSWORD", "env-secret")

	cfg := loadConfigFromYAML(t, explicitConfigYAML())
	if cfg.HTTP.Port != 19090 {
		t.Fatalf("HTTP.Port = %d, want 19090", cfg.HTTP.Port)
	}
	if cfg.Database.Postgres.Password != "env-secret" {
		t.Fatalf("Postgres.Password = %q, want env-secret", cfg.Database.Postgres.Password)
	}
}

func TestLoadAllowsOmittedOptionalConfigFields(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSection(`http:
  host: 127.0.0.1
  port: 18080
  read_timeout: 10s
  write_timeout: 10s
  idle_timeout: 60s
  shutdown_timeout: 10s`))
	if len(cfg.HTTP.TrustedProxies) != 0 {
		t.Fatalf("HTTP.TrustedProxies = %v, want empty", cfg.HTTP.TrustedProxies)
	}

	cfg = loadConfigFromYAML(t, configYAMLWithSection(`redis:
  addr: 127.0.0.1:6379
  db: 2
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s`))
	if cfg.Redis.Username != "" {
		t.Fatalf("Redis.Username = %q, want empty", cfg.Redis.Username)
	}
	if cfg.Redis.Password != "" {
		t.Fatalf("Redis.Password = %q, want empty", cfg.Redis.Password)
	}

	cfg = loadConfigFromYAML(t, configYAMLWithSection(`database:
  postgres:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    user_db_name: aegiscore_user
    pay_db_name: aegiscore_pay
    common_db_name: aegiscore_common
    driver: pgx
    sslmode: disable
    max_open_conns: 20
    max_idle_conns: 4
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s`))
	if cfg.Database.Postgres.Password != "" {
		t.Fatalf("Postgres.Password = %q, want empty", cfg.Database.Postgres.Password)
	}

	cfg = loadConfigFromYAML(t, configYAMLWithSection(`database:
  postgres:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    user_db_name: aegiscore_user
    common_db_name: aegiscore_common
    driver: pgx
    sslmode: disable
    max_open_conns: 20
    max_idle_conns: 4
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s`))
	if cfg.Database.Postgres.PayDBName != "" {
		t.Fatalf("Postgres.PayDBName = %q, want empty", cfg.Database.Postgres.PayDBName)
	}
	if _, ok := cfg.Postgres("pay_db"); ok {
		t.Fatal("Postgres(pay_db) ok = true")
	}
}

func TestPostgresNamedDatabaseDSNs(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSection(`database:
  postgres:
    host: db.example.internal
    port: 15432
    username: user@example.com
    password: p@ss/w:rd
    user_db_name: user_db
    pay_db_name: pay_db
    common_db_name: common_db
    driver: pgx
    sslmode: disable
    max_open_conns: 20
    max_idle_conns: 4
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s`))

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

func explicitConfigYAML() string {
	return `app:
  name: aegiscore-test
  environment: test

http:
  host: 127.0.0.1
  port: 18080
  read_timeout: 10s
  write_timeout: 10s
  idle_timeout: 60s
  shutdown_timeout: 10s
  trusted_proxies:
    - 127.0.0.1

log:
  level: info
  format: json

redis:
  addr: 127.0.0.1:6379
  username: ""
  password: ""
  db: 2
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s

database:
  postgres:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    password: secret
    user_db_name: aegiscore_user
    pay_db_name: aegiscore_pay
    common_db_name: aegiscore_common
    driver: pgx
    sslmode: disable
    max_open_conns: 20
    max_idle_conns: 4
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s
`
}

func configYAMLWithSection(section string) string {
	sections := map[string]string{
		"app": `app:
  name: aegiscore-test
  environment: test`,
		"http": `http:
  host: 127.0.0.1
  port: 18080
  read_timeout: 10s
  write_timeout: 10s
  idle_timeout: 60s
  shutdown_timeout: 10s
  trusted_proxies:
    - 127.0.0.1`,
		"log": `log:
  level: info
  format: json`,
		"redis": `redis:
  addr: 127.0.0.1:6379
  username: ""
  password: ""
  db: 2
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s`,
		"database": `database:
  postgres:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    password: secret
    user_db_name: aegiscore_user
    pay_db_name: aegiscore_pay
    common_db_name: aegiscore_common
    driver: pgx
    sslmode: disable
    max_open_conns: 20
    max_idle_conns: 4
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s`,
	}
	for name := range sections {
		if strings.HasPrefix(section, name+":") {
			sections[name] = section
			break
		}
	}
	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s\n", sections["app"], sections["http"], sections["log"], sections["redis"], sections["database"])
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
