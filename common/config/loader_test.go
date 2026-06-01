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
	if cfg.Log.Directory != "./logs" {
		t.Fatalf("Log.Directory = %q, want ./logs", cfg.Log.Directory)
	}
	if cfg.Log.Filename != "aegiscore-test" {
		t.Fatalf("Log.Filename = %q, want aegiscore-test", cfg.Log.Filename)
	}
	if !cfg.Log.Console {
		t.Fatal("Log.Console = false, want true")
	}
	if cfg.Log.MaxAgeDays != 7 || cfg.Log.MaxSizeMB != 100 || cfg.Log.MaxBackups != 30 {
		t.Fatalf("Log rotation = (%d,%d,%d), want (7,100,30)", cfg.Log.MaxAgeDays, cfg.Log.MaxSizeMB, cfg.Log.MaxBackups)
	}
	cacheRedis, ok := cfg.RedisConfig("cache_redis")
	if !ok {
		t.Fatal("RedisConfig(cache_redis) ok = false")
	}
	if cacheRedis.DB != 2 {
		t.Fatalf("cache_redis.DB = %d, want 2", cacheRedis.DB)
	}
	if cacheRedis.PingTimeout != 7*time.Second {
		t.Fatalf("cache_redis.PingTimeout = %s, want 7s", cacheRedis.PingTimeout)
	}
	queueRedis, ok := cfg.RedisConfig("queue_redis")
	if !ok {
		t.Fatal("RedisConfig(queue_redis) ok = false")
	}
	if queueRedis.DB != 1 {
		t.Fatalf("queue_redis.DB = %d, want 1", queueRedis.DB)
	}

	pg := cfg.PostgresConfigs["user_db"]
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
	if pg.DBName != "aegiscore_user" {
		t.Fatalf("DBName = %q, want aegiscore_user", pg.DBName)
	}
	if cfg.PostgresConfigs["pay_db"].DBName != "aegiscore_pay" {
		t.Fatalf("pay_db.DBName = %q, want aegiscore_pay", cfg.PostgresConfigs["pay_db"].DBName)
	}
}

func TestLoadDoesNotValidateMissingPrimaryConfigFields(t *testing.T) {
	cfg := loadConfigFromYAML(t, `app:
  environment: test

http:
  port: 0

log: {}

redis:
  cache_redis:
    db: 0

postgres:
  user_db:
    port: 0
`)

	if cfg.App.Name != "" {
		t.Fatalf("App.Name = %q, want empty", cfg.App.Name)
	}
	if cfg.HTTP.Host != "" {
		t.Fatalf("HTTP.Host = %q, want empty", cfg.HTTP.Host)
	}
	if cfg.HTTP.Port != 0 {
		t.Fatalf("HTTP.Port = %d, want 0", cfg.HTTP.Port)
	}
	if cfg.Redis["cache_redis"].Addr != "" {
		t.Fatalf("cache_redis.Addr = %q, want empty", cfg.Redis["cache_redis"].Addr)
	}
	if cfg.PostgresConfigs["user_db"].Host != "" {
		t.Fatalf("user_db.Host = %q, want empty", cfg.PostgresConfigs["user_db"].Host)
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
		t.Fatalf("HTTP.ReadTimeout = %s, want 0", cfg.HTTP.ReadTimeout)
	}

	cfg = loadConfigFromYAML(t, configYAMLWithSection(`redis:
  cache_redis:
    addr: 127.0.0.1:6379
    db: -1
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    ping_timeout: 0s`))
	if cfg.Redis["cache_redis"].DB != -1 {
		t.Fatalf("cache_redis.DB = %d, want -1", cfg.Redis["cache_redis"].DB)
	}

	cfg = loadConfigFromYAML(t, configYAMLWithSection(`postgres:
  user_db:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    password: secret
    db_name: aegiscore_user
    driver: pgx
    sslmode: disable
    max_open_conns: 0
    max_idle_conns: 0
    conn_max_lifetime: 0s
    conn_max_idle_time: 0s
    ping_timeout: 0s`))
	if cfg.PostgresConfigs["user_db"].MaxOpenConns != 0 {
		t.Fatalf("user_db.MaxOpenConns = %d, want 0", cfg.PostgresConfigs["user_db"].MaxOpenConns)
	}
	if cfg.PostgresConfigs["user_db"].PingTimeout != 0 {
		t.Fatalf("user_db.PingTimeout = %s, want 0", cfg.PostgresConfigs["user_db"].PingTimeout)
	}
}

func TestLoadEnvironmentOverride(t *testing.T) {
	t.Setenv("AEGISCORE_HTTP_PORT", "19090")
	t.Setenv("AEGISCORE_HTTP_READ_TIMEOUT", "30s")
	t.Setenv("AEGISCORE_HTTP_WRITE_TIMEOUT", "60s")
	t.Setenv("AEGISCORE_HTTP_IDLE_TIMEOUT", "120s")
	t.Setenv("AEGISCORE_HTTP_SHUTDOWN_TIMEOUT", "25s")
	t.Setenv("AEGISCORE_REDIS_CACHE_REDIS_DB", "9")
	t.Setenv("AEGISCORE_POSTGRES_USER_DB_PASSWORD", "env-secret")
	t.Setenv("AEGISCORE_POSTGRES_USER_DB_MAX_OPEN_CONNS", "30")

	cfg := loadConfigFromYAML(t, explicitConfigYAML())
	if cfg.HTTP.Port != 19090 {
		t.Fatalf("HTTP.Port = %d, want 19090", cfg.HTTP.Port)
	}
	if cfg.HTTP.ReadTimeout != 30*time.Second || cfg.HTTP.WriteTimeout != 60*time.Second || cfg.HTTP.IdleTimeout != 120*time.Second || cfg.HTTP.ShutdownTimeout != 25*time.Second {
		t.Fatalf("HTTP timeouts = (%s,%s,%s,%s), want (30s,60s,120s,25s)", cfg.HTTP.ReadTimeout, cfg.HTTP.WriteTimeout, cfg.HTTP.IdleTimeout, cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Redis["cache_redis"].DB != 9 {
		t.Fatalf("cache_redis.DB = %d, want 9", cfg.Redis["cache_redis"].DB)
	}
	if cfg.PostgresConfigs["user_db"].Password != "env-secret" {
		t.Fatalf("user_db.Password = %q, want env-secret", cfg.PostgresConfigs["user_db"].Password)
	}
	if cfg.PostgresConfigs["user_db"].MaxOpenConns != 30 {
		t.Fatalf("user_db.MaxOpenConns = %d, want 30", cfg.PostgresConfigs["user_db"].MaxOpenConns)
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
  cache_redis:
    addr: 127.0.0.1:6379
    db: 2
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s`))
	if cfg.Redis["cache_redis"].Username != "" {
		t.Fatalf("Redis.Username = %q, want empty", cfg.Redis["cache_redis"].Username)
	}
	if cfg.Redis["cache_redis"].Password != "" {
		t.Fatalf("Redis.Password = %q, want empty", cfg.Redis["cache_redis"].Password)
	}

	cfg = loadConfigFromYAML(t, configYAMLWithSection(`postgres:
  user_db:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    db_name: aegiscore_user
    driver: pgx
    sslmode: disable
    max_open_conns: 20
    max_idle_conns: 4
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s`))
	if cfg.PostgresConfigs["user_db"].Password != "" {
		t.Fatalf("Postgres.Password = %q, want empty", cfg.PostgresConfigs["user_db"].Password)
	}
	if _, ok := cfg.Postgres("pay_db"); ok {
		t.Fatal("Postgres(pay_db) ok = true")
	}
}

func TestLoadYAMLMergeForNamedDatastores(t *testing.T) {
	cfg := loadConfigFromYAML(t, explicitConfigYAML())

	if got := cfg.Redis["queue_redis"].DialTimeout; got != 10*time.Second {
		t.Fatalf("queue_redis.DialTimeout = %s, want 10s", got)
	}
	if got := cfg.Redis["queue_redis"].ReadTimeout; got != 3*time.Second {
		t.Fatalf("queue_redis.ReadTimeout = %s, want 3s", got)
	}
	if got := cfg.PostgresConfigs["user_db"].MaxOpenConns; got != 20 {
		t.Fatalf("user_db.MaxOpenConns = %d, want 20", got)
	}
	if got := cfg.PostgresConfigs["pay_db"].MaxOpenConns; got != 25 {
		t.Fatalf("pay_db.MaxOpenConns = %d, want 25", got)
	}
}

func TestPostgresNamedDatabaseDSNs(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSection(`postgres:
  user_db:
    host: db.example.internal
    port: 15432
    username: user@example.com
    password: p@ss/w:rd
    db_name: user_db
    driver: pgx
    sslmode: disable
    max_open_conns: 20
    max_idle_conns: 4
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s
  common_db:
    host: db.example.internal
    port: 15432
    username: user@example.com
    password: p@ss/w:rd
    db_name: common_db
    driver: pgx
    sslmode: disable
    max_open_conns: 20
    max_idle_conns: 4
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s
  pay_db:
    host: db.example.internal
    port: 15432
    username: user@example.com
    password: p@ss/w:rd
    db_name: pay_db
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

func TestRedisConfigLookup(t *testing.T) {
	cfg := loadConfigFromYAML(t, explicitConfigYAML())
	redisCfg, ok := cfg.RedisConfig("cache_redis")
	if !ok {
		t.Fatal("RedisConfig(cache_redis) ok = false")
	}
	if redisCfg.Addr != "127.0.0.1:6379" {
		t.Fatalf("Addr = %q, want 127.0.0.1:6379", redisCfg.Addr)
	}
	if _, ok := cfg.RedisConfig("missing_redis"); ok {
		t.Fatal("RedisConfig(missing_redis) ok = true")
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
  directory: ./logs
  filename: aegiscore-test
  console: true
  max_age_days: 7
  max_size_mb: 100
  max_backups: 30

.redis_base: &redis_base
  addr: 127.0.0.1:6379
  username: ""
  password: ""
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
  ping_timeout: 7s

.postgres_base: &postgres_base
  host: 127.0.0.1
  port: 15432
  username: aegiscore
  password: secret
  driver: pgx
  sslmode: disable
  max_open_conns: 25
  max_idle_conns: 4
  conn_max_lifetime: 45m
  conn_max_idle_time: 12m
  ping_timeout: 7s

redis:
  cache_redis:
    <<: *redis_base
    db: 2
  queue_redis:
    <<: *redis_base
    db: 1
    dial_timeout: 10s
    ping_timeout: 9s

postgres:
  user_db:
    <<: *postgres_base
    db_name: aegiscore_user
    max_open_conns: 20
  pay_db:
    <<: *postgres_base
    db_name: aegiscore_pay
  common_db:
    <<: *postgres_base
    db_name: aegiscore_common
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
  format: json
  directory: ./logs
  filename: aegiscore-test
  console: true
  max_age_days: 7
  max_size_mb: 100
  max_backups: 30`,
		"redis": `redis:
  cache_redis:
    addr: 127.0.0.1:6379
    username: ""
    password: ""
    db: 2
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    ping_timeout: 7s`,
		"postgres": `postgres:
  user_db:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    password: secret
    db_name: aegiscore_user
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
	ordered := []string{sections["app"], sections["http"], sections["log"], sections["redis"], sections["postgres"]}
	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s\n", ordered[0], ordered[1], ordered[2], ordered[3], ordered[4])
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
