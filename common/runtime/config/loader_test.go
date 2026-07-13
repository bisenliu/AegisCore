package config

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadExplicitConfig(t *testing.T) {
	cfg := loadConfigFromYAML(t, explicitConfigYAML())

	require.Equal(t, "aegiscore-test", cfg.App.Name)
	require.Equal(t, "Asia/Shanghai", cfg.System.Timezone)
	require.Equal(t, 18080, cfg.HTTP.Port)
	require.True(t, cfg.HTTP.Pprof.Enabled)
	require.Equal(t, "/internal/debug/pprof", cfg.HTTP.Pprof.BasePath)
	authTokenVersion := requireLocalCacheInstance(t, cfg, "auth_token_version")
	require.Equal(t, int64(1000), authTokenVersion.Capacity)
	require.Equal(t, time.Second, authTokenVersion.TTL)
	require.Equal(t, 300*time.Millisecond, authTokenVersion.LoadTimeout)
	require.Equal(t, int64(2000), authTokenVersion.NumCounters)
	require.Equal(t, int64(128), authTokenVersion.BufferItems)
	require.Equal(t, "./logs", cfg.Log.Directory)
	require.Equal(t, "aegiscore-test", cfg.Log.Filename)
	require.True(t, cfg.Observability.Metrics.Enabled)
	require.Equal(t, "/metrics", cfg.Observability.Metrics.Path)
	require.Equal(t, "otlp", cfg.Observability.Tracing.Exporter)
	require.Equal(t, "collector:4317", cfg.Observability.Tracing.OTLPEndpoint)
	cacheRedis, ok := cfg.RedisConfig("cache_redis")
	require.True(t, ok)
	require.Equal(t, 2, cacheRedis.DB)
	pg := cfg.Postgres["user_db"]
	require.Equal(t, "127.0.0.1", pg.Host)
	require.Equal(t, 15432, pg.Port)
	require.Equal(t, "pgx", pg.Driver)
	require.Equal(t, "aegiscore_user", pg.DBName)
}

func TestLoadIntoServiceExtension(t *testing.T) {
	type extended struct {
		Config `mapstructure:",squash"`
		Auth   struct {
			JWT struct {
				Secret string `mapstructure:"secret"`
			} `mapstructure:"jwt"`
		} `mapstructure:"auth"`
	}
	cfg := loadIntoFromYAML[extended](t, explicitConfigYAML(), func(cfg extended) error { return cfg.Validate() })
	require.Equal(t, "test-secret", cfg.Auth.JWT.Secret)
	require.Equal(t, "aegiscore-test", cfg.App.Name)
}

func TestLoadValidatesMissingPrimaryConfigFields(t *testing.T) {
	err := loadConfigErrorFromYAML(t, `app:
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

	assertConfigLoadErrorContains(t, err,
		"system.timezone is required",
		"app.name is required",
		"http.host is required",
		"http.port must be between 1 and 65535",
		"redis.cache_redis.addr is required",
		"postgres.user_db.host is required",
	)
}

func TestLoadValidatesInvalidBasicValues(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`http:
  host: 127.0.0.1
  port: 70000
  read_timeout: 0s
  write_timeout: 0s
  idle_timeout: 0s
  shutdown_timeout: 0s`))
	assertConfigLoadErrorContains(t, err,
		"http.port must be between 1 and 65535",
		"http.read_timeout must be > 0",
		"http.write_timeout must be > 0",
		"http.idle_timeout must be > 0",
		"http.shutdown_timeout must be > 0",
	)
}

func TestLoadRejectsProductionLikeInsecureConfig(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSections(`app:
  name: aegiscore-test
  environment: prod`, `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 1.0
    exporter: otlp
    otlp_endpoint: collector.internal:4317
    insecure: true`))

	assertConfigLoadErrorContains(t, err,
		"postgres.user_db.sslmode must not be disable in production-like environments",
		"observability.tracing.insecure must not be true with otlp exporter in production-like environments",
	)
}

func TestLoadEnvironmentOverride(t *testing.T) {
	t.Setenv("AEGISCORE_SYSTEM_TIMEZONE", "UTC")
	t.Setenv("AEGISCORE_HTTP_PORT", "19090")
	t.Setenv("AEGISCORE_LOCAL_CACHE_AUTH_TOKEN_VERSION_CAPACITY", "3000")
	t.Setenv("AEGISCORE_OBSERVABILITY_METRICS_ENABLED", "false")
	t.Setenv("AEGISCORE_REDIS_CACHE_REDIS_DB", "9")
	t.Setenv("AEGISCORE_POSTGRES_USER_DB_PASSWORD", "env-secret")

	cfg := loadConfigFromYAML(t, explicitConfigYAML())
	require.Equal(t, "UTC", cfg.System.Timezone)
	require.Equal(t, 19090, cfg.HTTP.Port)
	require.Equal(t, int64(3000), requireLocalCacheInstance(t, cfg, "auth_token_version").Capacity)
	require.False(t, cfg.Observability.Metrics.Enabled)
	require.Equal(t, 9, cfg.Redis["cache_redis"].DB)
	require.Equal(t, "env-secret", cfg.Postgres["user_db"].Password)
}

func TestLoadValidatesLocalCacheConfig(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`local_cache:
  auth_token_version:
    capacity: 0
    ttl: 0s
    load_timeout: 0s
    num_counters: -1
    buffer_items: -1`))

	assertConfigLoadErrorContains(t, err,
		"local_cache.auth_token_version.capacity must be > 0",
		"local_cache.auth_token_version.ttl must be > 0",
		"local_cache.auth_token_version.load_timeout must be > 0",
		"local_cache.auth_token_version.num_counters must be >= 0",
		"local_cache.auth_token_version.buffer_items must be >= 0",
	)
}

func TestPostgresDatabaseConfigBuildsDSN(t *testing.T) {
	cfg := loadConfigFromYAML(t, explicitConfigYAML())
	dbCfg, ok := cfg.PostgresDatabaseConfig("user_db")
	require.True(t, ok)
	parsed, err := url.Parse(dbCfg.DSN)
	require.NoError(t, err)
	require.Equal(t, "postgres", parsed.Scheme)
	require.Equal(t, "127.0.0.1:15432", parsed.Host)
	require.Equal(t, "/aegiscore_user", parsed.Path)
	require.Equal(t, "disable", parsed.Query().Get("sslmode"))
}

func requireLocalCacheInstance(t *testing.T, cfg *Config, name string) LocalCacheInstanceConfig {
	t.Helper()
	cacheCfg, ok := cfg.LocalCache.Instance(name)
	require.True(t, ok)
	return cacheCfg
}

func loadConfigFromYAML(t *testing.T, content string) *Config {
	t.Helper()
	return loadIntoFromYAML(t, content, Config.Validate)
}

func loadConfigErrorFromYAML(t *testing.T, content string) error {
	t.Helper()
	path := writeTempConfig(t, content)
	_, err := Load(path)
	require.Error(t, err)
	return err
}

func loadIntoFromYAML[T any](t *testing.T, content string, validate func(T) error) *T {
	t.Helper()
	path := writeTempConfig(t, content)
	cfg, err := LoadInto(path, validate)
	require.NoError(t, err)
	return cfg
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func assertConfigLoadErrorContains(t *testing.T, err error, parts ...string) {
	t.Helper()
	for _, part := range parts {
		require.Contains(t, err.Error(), part)
	}
}

func configYAMLWithSection(section string) string {
	return configYAMLWithSections(section)
}

func configYAMLWithSections(sections ...string) string {
	content := explicitConfigYAML()
	for _, section := range sections {
		name := sectionName(section)
		content = replaceTopLevelSection(content, name, section)
	}
	return content
}

func sectionName(section string) string {
	line := strings.SplitN(section, "\n", 2)[0]
	return strings.TrimSuffix(strings.TrimSpace(line), ":")
}

func replaceTopLevelSection(content string, name string, replacement string) string {
	lines := strings.Split(content, "\n")
	start := -1
	end := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(line, name+":") {
			start = i
			continue
		}
		if start >= 0 && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			end = i
			break
		}
	}
	if start < 0 {
		return content + "\n" + replacement + "\n"
	}
	repl := strings.Split(replacement, "\n")
	updated := append([]string{}, lines[:start]...)
	updated = append(updated, repl...)
	updated = append(updated, lines[end:]...)
	return strings.Join(updated, "\n")
}

func explicitConfigYAML() string {
	return `app:
  name: aegiscore-test
  environment: local
system:
  timezone: Asia/Shanghai
http:
  host: 127.0.0.1
  port: 18080
  read_timeout: 10s
  write_timeout: 20s
  idle_timeout: 30s
  shutdown_timeout: 5s
  trusted_proxies: 127.0.0.1,10.0.0.1
  pprof:
    enabled: true
    base_path: /internal/debug/pprof
auth:
  jwt:
    secret: test-secret
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 15m
    refresh_token_ttl: 168h
    password_change_token_ttl: 5m
  password_kdf:
    argon2_concurrency: 2
    argon2_queue_size: 16
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5
ent:
  sql_debug: true
local_cache:
  auth_token_version:
    capacity: 1000
    ttl: 1s
    load_timeout: 300ms
    num_counters: 2000
    buffer_items: 128
log:
  level: info
  format: json
  directory: ./logs
  filename: aegiscore-test
  console: true
  max_age_days: 7
  max_size_mb: 100
  max_backups: 30
observability:
  metrics:
    enabled: true
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 0.25
    exporter: otlp
    otlp_endpoint: collector:4317
    insecure: false
redis:
  cache_redis:
    addr: 127.0.0.1:6379
    db: 2
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    ping_timeout: 7s
postgres:
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
    ping_timeout: 7s
`
}
