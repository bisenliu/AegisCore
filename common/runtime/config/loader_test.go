package config

import (
	"errors"
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
	if cfg.System.Timezone != "Asia/Shanghai" {
		t.Fatalf("System.Timezone = %q, want Asia/Shanghai", cfg.System.Timezone)
	}
	if cfg.HTTP.Port != 18080 {
		t.Fatalf("HTTP.Port = %d, want 18080", cfg.HTTP.Port)
	}
	if !cfg.HTTP.Pprof.Enabled || cfg.HTTP.Pprof.BasePath != "/internal/debug/pprof" {
		t.Fatalf("HTTP.Pprof = %#v, want enabled /internal/debug/pprof", cfg.HTTP.Pprof)
	}
	if cfg.Auth.JWT.Secret != "test-secret" {
		t.Fatalf("Auth.JWT.Secret = %q, want test-secret", cfg.Auth.JWT.Secret)
	}
	if cfg.Auth.JWT.Issuer != "aegiscore-test" || cfg.Auth.JWT.Audience != "aegiscore-users" {
		t.Fatalf("Auth.JWT issuer/audience = (%q,%q), want (aegiscore-test,aegiscore-users)", cfg.Auth.JWT.Issuer, cfg.Auth.JWT.Audience)
	}
	if cfg.Auth.JWT.AccessTokenTTL != 15*time.Minute {
		t.Fatalf("Auth.JWT.AccessTokenTTL = %s, want 15m", cfg.Auth.JWT.AccessTokenTTL)
	}
	if cfg.Auth.JWT.RefreshTokenTTL != 168*time.Hour {
		t.Fatalf("Auth.JWT.RefreshTokenTTL = %s, want 168h", cfg.Auth.JWT.RefreshTokenTTL)
	}
	if cfg.Auth.TokenVersionCacheTTL != 30*time.Second {
		t.Fatalf("Auth.TokenVersionCacheTTL = %s, want 30s", cfg.Auth.TokenVersionCacheTTL)
	}
	if !cfg.Auth.RefreshTokenRotation {
		t.Fatal("Auth.RefreshTokenRotation = false, want true")
	}
	if cfg.Auth.MaxActiveSessionsPerUser != 5 {
		t.Fatalf("Auth.MaxActiveSessionsPerUser = %d, want 5", cfg.Auth.MaxActiveSessionsPerUser)
	}
	authTokenVersion := requireLocalCacheInstance(t, cfg, "auth_token_version")
	if authTokenVersion.Capacity != 1000 || authTokenVersion.TTL != time.Second || authTokenVersion.LoadTimeout != 300*time.Millisecond {
		t.Fatalf("LocalCache[auth_token_version] = %#v, want capacity 1000 ttl 1s load timeout 300ms", authTokenVersion)
	}
	if authTokenVersion.NumCounters != 2000 || authTokenVersion.BufferItems != 128 {
		t.Fatalf("LocalCache[auth_token_version] tuning = (%d,%d), want (2000,128)", authTokenVersion.NumCounters, authTokenVersion.BufferItems)
	}
	rbacUserRoles := requireLocalCacheInstance(t, cfg, "rbac_user_roles")
	if rbacUserRoles.Capacity != 2000 || rbacUserRoles.TTL != 5*time.Second || rbacUserRoles.LoadTimeout != 500*time.Millisecond {
		t.Fatalf("LocalCache[rbac_user_roles] = %#v, want capacity 2000 ttl 5s load timeout 500ms", rbacUserRoles)
	}
	backgroundJobs := requireLocalCacheInstance(t, cfg, "background_jobs")
	if backgroundJobs.Capacity != 300 || backgroundJobs.TTL != 10*time.Second || backgroundJobs.LoadTimeout != 200*time.Millisecond {
		t.Fatalf("LocalCache[background_jobs] = %#v, want capacity 300 ttl 10s load timeout 200ms", backgroundJobs)
	}
	if !cfg.Ent.SQLDebug {
		t.Fatal("Ent.SQLDebug = false, want true")
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
	if !cfg.Observability.Metrics.Enabled {
		t.Fatal("Observability.Metrics.Enabled = false, want true")
	}
	if cfg.Observability.Metrics.Path != "/metrics" || !cfg.Observability.Metrics.IncludeRuntime {
		t.Fatalf("Observability.Metrics = %#v, want path /metrics with runtime metrics", cfg.Observability.Metrics)
	}
	if !cfg.Observability.Tracing.Enabled {
		t.Fatal("Observability.Tracing.Enabled = false, want true")
	}
	if cfg.Observability.Tracing.SampleRatio != 0.25 || cfg.Observability.Tracing.Exporter != "otlp" || cfg.Observability.Tracing.OTLPEndpoint != "collector:4317" || cfg.Observability.Tracing.Insecure {
		t.Fatalf("Observability.Tracing = %#v, want otlp collector config", cfg.Observability.Tracing)
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

	pg := cfg.Postgres["user_db"]
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
	if cfg.Postgres["pay_db"].DBName != "aegiscore_pay" {
		t.Fatalf("pay_db.DBName = %q, want aegiscore_pay", cfg.Postgres["pay_db"].DBName)
	}
}

func requireLocalCacheInstance(t *testing.T, cfg *Config, name string) LocalCacheInstanceConfig {
	t.Helper()
	cacheCfg, ok := cfg.LocalCache.Instance(name)
	if !ok {
		t.Fatalf("LocalCache.Instance(%q) ok = false", name)
	}
	return cacheCfg
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
		"auth.jwt.secret is required",
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

	err = loadConfigErrorFromYAML(t, configYAMLWithSection(`redis:
  cache_redis:
    addr: 127.0.0.1
    db: -1
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    ping_timeout: 0s`))
	assertConfigLoadErrorContains(t, err,
		"redis.cache_redis.addr must be in host:port format",
		"redis.cache_redis.db must be >= 0",
		"redis.cache_redis.ping_timeout must be > 0",
	)

	err = loadConfigErrorFromYAML(t, configYAMLWithSection(`postgres:
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
	assertConfigLoadErrorContains(t, err,
		"postgres.user_db.max_open_conns must be > 0",
		"postgres.user_db.conn_max_lifetime must be > 0",
		"postgres.user_db.conn_max_idle_time must be > 0",
		"postgres.user_db.ping_timeout must be > 0",
	)
}

func TestLoadAggregatesConfigValidationErrors(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`postgres:
  user_db:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    password: secret
    db_name: aegiscore_user
    driver: pgx
    sslmode: disable
    max_open_conns: 2
    max_idle_conns: 3
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s`))

	assertConfigLoadErrorContains(t, err, "postgres.user_db.max_idle_conns must be <= max_open_conns")
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Load error = %T, want ValidationError in chain", err)
	}
	if got := len(validationErr.Unwrap()); got != 1 {
		t.Fatalf("validation error count = %d, want 1", got)
	}
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
		"auth.jwt.secret must not use a development default in production-like environments",
		"postgres.user_db.sslmode must not be disable in production-like environments",
		"observability.tracing.insecure must not be true with otlp exporter in production-like environments",
	)
	if strings.Contains(err.Error(), "test-secret") {
		t.Fatalf("Load error leaks sensitive JWT secret: %q", err.Error())
	}
	if strings.Contains(err.Error(), "collector.internal:4317") {
		t.Fatalf("Load error leaks OTLP endpoint: %q", err.Error())
	}
}

func TestLoadEnvironmentOverride(t *testing.T) {
	t.Setenv("AEGISCORE_SYSTEM_TIMEZONE", "UTC")
	t.Setenv("AEGISCORE_HTTP_PORT", "19090")
	t.Setenv("AEGISCORE_HTTP_READ_TIMEOUT", "30s")
	t.Setenv("AEGISCORE_HTTP_WRITE_TIMEOUT", "60s")
	t.Setenv("AEGISCORE_HTTP_IDLE_TIMEOUT", "120s")
	t.Setenv("AEGISCORE_HTTP_SHUTDOWN_TIMEOUT", "25s")
	t.Setenv("AEGISCORE_AUTH_JWT_SECRET", "env-secret")
	t.Setenv("AEGISCORE_AUTH_JWT_ISSUER", "env-issuer")
	t.Setenv("AEGISCORE_AUTH_JWT_REFRESH_TOKEN_TTL", "720h")
	t.Setenv("AEGISCORE_AUTH_TOKEN_VERSION_CACHE_TTL", "30s")
	t.Setenv("AEGISCORE_AUTH_MAX_ACTIVE_SESSIONS_PER_USER", "7")
	t.Setenv("AEGISCORE_LOCAL_CACHE_AUTH_TOKEN_VERSION_CAPACITY", "3000")
	t.Setenv("AEGISCORE_LOCAL_CACHE_AUTH_TOKEN_VERSION_TTL", "2s")
	t.Setenv("AEGISCORE_LOCAL_CACHE_AUTH_TOKEN_VERSION_LOAD_TIMEOUT", "400ms")
	t.Setenv("AEGISCORE_LOCAL_CACHE_RBAC_USER_ROLES_CAPACITY", "4000")
	t.Setenv("AEGISCORE_LOCAL_CACHE_RBAC_USER_ROLES_BUFFER_ITEMS", "256")
	t.Setenv("AEGISCORE_LOCAL_CACHE_BACKGROUND_JOBS_TTL", "15s")
	t.Setenv("AEGISCORE_ENT_SQL_DEBUG", "false")
	t.Setenv("AEGISCORE_OBSERVABILITY_METRICS_ENABLED", "false")
	t.Setenv("AEGISCORE_OBSERVABILITY_METRICS_PATH", "/internal/metrics")
	t.Setenv("AEGISCORE_OBSERVABILITY_METRICS_INCLUDE_RUNTIME", "false")
	t.Setenv("AEGISCORE_OBSERVABILITY_TRACING_ENABLED", "false")
	t.Setenv("AEGISCORE_OBSERVABILITY_TRACING_SAMPLE_RATIO", "0.5")
	t.Setenv("AEGISCORE_OBSERVABILITY_TRACING_EXPORTER", "none")
	t.Setenv("AEGISCORE_OBSERVABILITY_TRACING_OTLP_ENDPOINT", "env-collector:4317")
	t.Setenv("AEGISCORE_OBSERVABILITY_TRACING_INSECURE", "true")
	t.Setenv("AEGISCORE_REDIS_CACHE_REDIS_DB", "9")
	t.Setenv("AEGISCORE_POSTGRES_USER_DB_PASSWORD", "env-secret")
	t.Setenv("AEGISCORE_POSTGRES_USER_DB_MAX_OPEN_CONNS", "30")

	cfg := loadConfigFromYAML(t, explicitConfigYAML())
	if cfg.System.Timezone != "UTC" {
		t.Fatalf("System.Timezone = %q, want UTC", cfg.System.Timezone)
	}
	if cfg.HTTP.Port != 19090 {
		t.Fatalf("HTTP.Port = %d, want 19090", cfg.HTTP.Port)
	}
	if cfg.HTTP.ReadTimeout != 30*time.Second || cfg.HTTP.WriteTimeout != 60*time.Second || cfg.HTTP.IdleTimeout != 120*time.Second || cfg.HTTP.ShutdownTimeout != 25*time.Second {
		t.Fatalf("HTTP timeouts = (%s,%s,%s,%s), want (30s,60s,120s,25s)", cfg.HTTP.ReadTimeout, cfg.HTTP.WriteTimeout, cfg.HTTP.IdleTimeout, cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Auth.JWT.Secret != "env-secret" || cfg.Auth.JWT.Issuer != "env-issuer" {
		t.Fatalf("Auth JWT = %#v, want env overrides", cfg.Auth.JWT)
	}
	if cfg.Auth.JWT.RefreshTokenTTL != 720*time.Hour {
		t.Fatalf("Auth.JWT.RefreshTokenTTL = %s, want 720h", cfg.Auth.JWT.RefreshTokenTTL)
	}
	if cfg.Auth.TokenVersionCacheTTL != 30*time.Second {
		t.Fatalf("Auth.TokenVersionCacheTTL = %s, want 30s", cfg.Auth.TokenVersionCacheTTL)
	}
	if cfg.Auth.MaxActiveSessionsPerUser != 7 {
		t.Fatalf("Auth.MaxActiveSessionsPerUser = %d, want 7", cfg.Auth.MaxActiveSessionsPerUser)
	}
	authTokenVersion := requireLocalCacheInstance(t, cfg, "auth_token_version")
	if authTokenVersion.Capacity != 3000 || authTokenVersion.TTL != 2*time.Second || authTokenVersion.LoadTimeout != 400*time.Millisecond {
		t.Fatalf("LocalCache[auth_token_version] = %#v, want env overrides", authTokenVersion)
	}
	rbacUserRoles := requireLocalCacheInstance(t, cfg, "rbac_user_roles")
	if rbacUserRoles.Capacity != 4000 || rbacUserRoles.BufferItems != 256 {
		t.Fatalf("LocalCache[rbac_user_roles] = %#v, want env overrides", rbacUserRoles)
	}
	backgroundJobs := requireLocalCacheInstance(t, cfg, "background_jobs")
	if backgroundJobs.TTL != 15*time.Second {
		t.Fatalf("LocalCache[background_jobs].TTL = %s, want env override 15s", backgroundJobs.TTL)
	}
	if cfg.Ent.SQLDebug {
		t.Fatal("Ent.SQLDebug = true, want env override false")
	}
	if cfg.Observability.Metrics.Enabled {
		t.Fatal("Observability.Metrics.Enabled = true, want env override false")
	}
	if cfg.Observability.Metrics.Path != "/internal/metrics" {
		t.Fatalf("Observability.Metrics.Path = %q, want /internal/metrics", cfg.Observability.Metrics.Path)
	}
	if cfg.Observability.Metrics.IncludeRuntime {
		t.Fatal("Observability.Metrics.IncludeRuntime = true, want env override false")
	}
	if cfg.Observability.Tracing.Enabled || cfg.Observability.Tracing.SampleRatio != 0.5 || cfg.Observability.Tracing.Exporter != "none" || cfg.Observability.Tracing.OTLPEndpoint != "env-collector:4317" || !cfg.Observability.Tracing.Insecure {
		t.Fatalf("Observability.Tracing = %#v, want env overrides", cfg.Observability.Tracing)
	}
	if cfg.Redis["cache_redis"].DB != 9 {
		t.Fatalf("cache_redis.DB = %d, want 9", cfg.Redis["cache_redis"].DB)
	}
	if cfg.Postgres["user_db"].Password != "env-secret" {
		t.Fatalf("user_db.Password = %q, want env-secret", cfg.Postgres["user_db"].Password)
	}
	if cfg.Postgres["user_db"].MaxOpenConns != 30 {
		t.Fatalf("user_db.MaxOpenConns = %d, want 30", cfg.Postgres["user_db"].MaxOpenConns)
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
    write_timeout: 3s
    ping_timeout: 7s`))
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
	if cfg.Postgres["user_db"].Password != "" {
		t.Fatalf("Postgres.Password = %q, want empty", cfg.Postgres["user_db"].Password)
	}
	if _, ok := cfg.PostgresDatabaseConfig("pay_db"); ok {
		t.Fatal("PostgresDatabaseConfig(pay_db) ok = true")
	}
}

func TestLoadValidatesAuthSessionLimit(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSection(`auth:
  jwt:
    secret: test-secret
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 15m
    refresh_token_ttl: 168h
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: 0`))
	if cfg.Auth.MaxActiveSessionsPerUser != 0 {
		t.Fatalf("MaxActiveSessionsPerUser = %d, want 0", cfg.Auth.MaxActiveSessionsPerUser)
	}

	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`auth:
  jwt:
    secret: test-secret
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 15m
    refresh_token_ttl: 168h
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: -1`))
	assertConfigLoadErrorContains(t, err, "auth.max_active_sessions_per_user must be >= 0")
}

func TestLoadValidatesLocalCacheConfig(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`local_cache:
  auth_token_version:
    capacity: 0
    ttl: 0s
    load_timeout: 0s
    num_counters: -1
    buffer_items: -1
  rbac_user_roles:
    capacity: -1
    ttl: 0s
    load_timeout: 0s
    num_counters: -1
    buffer_items: -1
  background_jobs:
    capacity: -1
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
		"local_cache.rbac_user_roles.capacity must be > 0",
		"local_cache.rbac_user_roles.ttl must be > 0",
		"local_cache.rbac_user_roles.load_timeout must be > 0",
		"local_cache.rbac_user_roles.num_counters must be >= 0",
		"local_cache.rbac_user_roles.buffer_items must be >= 0",
		"local_cache.background_jobs.capacity must be > 0",
		"local_cache.background_jobs.ttl must be > 0",
		"local_cache.background_jobs.load_timeout must be > 0",
		"local_cache.background_jobs.num_counters must be >= 0",
		"local_cache.background_jobs.buffer_items must be >= 0",
	)
}

func TestConfigValidateRejectsEmptyLocalCacheName(t *testing.T) {
	err := Config{
		LocalCache: LocalCacheConfig{
			"": LocalCacheInstanceConfig{Capacity: 1, TTL: time.Second, LoadTimeout: time.Second},
		},
	}.validateLocalCache()

	if len(err) != 1 || err[0].Error() != "local_cache must not contain an empty named instance" {
		t.Fatalf("validateLocalCache() = %#v, want empty instance name error", err)
	}
}

func TestLoadValidatesObservabilityConfig(t *testing.T) {
	tests := []struct {
		name    string
		section string
		wants   []string
	}{
		{
			name: "empty metrics path",
			section: `observability:
  metrics:
    enabled: false
    path: ""
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 1.0
    exporter: none
    otlp_endpoint: ""
    insecure: false`,
			wants: []string{"observability.metrics.path is required"},
		},
		{
			name: "invalid enabled metrics path",
			section: `observability:
  metrics:
    enabled: true
    path: metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 1.0
    exporter: none
    otlp_endpoint: ""
    insecure: false`,
			wants: []string{"observability.metrics.path must start with / when metrics is enabled"},
		},
		{
			name: "sample ratio below range",
			section: `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: -0.1
    exporter: none
    otlp_endpoint: ""
    insecure: false`,
			wants: []string{"observability.tracing.sample_ratio must be between 0 and 1"},
		},
		{
			name: "sample ratio above range",
			section: `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 1.1
    exporter: none
    otlp_endpoint: ""
    insecure: false`,
			wants: []string{"observability.tracing.sample_ratio must be between 0 and 1"},
		},
		{
			name: "invalid tracing exporter",
			section: `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 1.0
    exporter: zipkin
    otlp_endpoint: ""
    insecure: false`,
			wants: []string{"observability.tracing.exporter must be one of none, otlp"},
		},
		{
			name: "otlp endpoint required",
			section: `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 1.0
    exporter: otlp
    otlp_endpoint: ""
    insecure: false`,
			wants: []string{"observability.tracing.otlp_endpoint is required when exporter is otlp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loadConfigErrorFromYAML(t, configYAMLWithSection(tt.section))
			assertConfigLoadErrorContains(t, err, tt.wants...)
		})
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
	if got := cfg.Postgres["user_db"].MaxOpenConns; got != 20 {
		t.Fatalf("user_db.MaxOpenConns = %d, want 20", got)
	}
	if got := cfg.Postgres["pay_db"].MaxOpenConns; got != 25 {
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
  audit_db:
    host: db.example.internal
    port: 15432
    username: user@example.com
    password: p@ss/w:rd
    db_name: audit_db
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
		{name: "audit_db", wantDBName: "audit_db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, ok := cfg.PostgresDatabaseConfig(tt.name)
			if !ok {
				t.Fatalf("PostgresDatabaseConfig(%q) ok = false", tt.name)
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

	if _, ok := cfg.PostgresDatabaseConfig("pay_db"); !ok {
		t.Fatal("PostgresDatabaseConfig(pay_db) ok = false")
	}
	if _, ok := cfg.PostgresDatabaseConfig("missing_db"); ok {
		t.Fatal("PostgresDatabaseConfig(missing_db) ok = true")
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
	return `system:
  timezone: Asia/Shanghai

app:
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
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5

ent:
  sql_debug: true

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

local_cache:
  auth_token_version:
    capacity: 1000
    ttl: 1s
    load_timeout: 300ms
    num_counters: 2000
    buffer_items: 128
  rbac_user_roles:
    capacity: 2000
    ttl: 5s
    load_timeout: 500ms
    num_counters: 0
    buffer_items: 0
  background_jobs:
    capacity: 300
    ttl: 10s
    load_timeout: 200ms
    num_counters: 1000
    buffer_items: 64

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
  audit_db:
    <<: *postgres_base
    db_name: aegiscore_audit
`
}

func configYAMLWithSection(section string) string {
	return configYAMLWithSections(section)
}

func configYAMLWithSections(overrides ...string) string {
	sections := map[string]string{
		"system": `system:
  timezone: Asia/Shanghai`,
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
    - 127.0.0.1
  pprof:
    enabled: false
    base_path: /debug/pprof`,
		"auth": `auth:
  jwt:
    secret: test-secret
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 15m
    refresh_token_ttl: 168h
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5`,
		"ent": `ent:
  sql_debug: false`,
		"log": `log:
  level: info
  format: json
  directory: ./logs
  filename: aegiscore-test
  console: true
  max_age_days: 7
  max_size_mb: 100
  max_backups: 30`,
		"observability": `observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 1.0
    exporter: none
    otlp_endpoint: ""
    insecure: false`,
		"local_cache": `local_cache:
  auth_token_version:
    capacity: 1000
    ttl: 1s
    load_timeout: 300ms
    num_counters: 0
    buffer_items: 0
  rbac_user_roles:
    capacity: 2000
    ttl: 5s
    load_timeout: 500ms
    num_counters: 0
    buffer_items: 0`,
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
	for _, override := range overrides {
		for name := range sections {
			if strings.HasPrefix(override, name+":") {
				sections[name] = override
				break
			}
		}
	}
	ordered := []string{sections["system"], sections["app"], sections["http"], sections["auth"], sections["local_cache"], sections["ent"], sections["log"], sections["observability"], sections["redis"], sections["postgres"]}
	return strings.Join(ordered, "\n\n") + "\n"
}

func loadConfigFromYAML(t *testing.T, content string) *Config {
	t.Helper()
	cfg, err := Load(writeTempConfig(t, content))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func loadConfigErrorFromYAML(t *testing.T, content string) error {
	t.Helper()
	_, err := Load(writeTempConfig(t, content))
	if err == nil {
		t.Fatal("Load error = nil, want validation error")
	}
	return err
}

func assertConfigLoadErrorContains(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("Load error = nil, want error")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Load error = %T, want ValidationError in chain: %v", err, err)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Load error = %q, want %q", err.Error(), want)
		}
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
