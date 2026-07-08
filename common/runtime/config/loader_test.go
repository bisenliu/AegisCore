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
	require.Equal(t, "test-secret", cfg.Auth.JWT.Secret)
	require.Equal(t, "aegiscore-test", cfg.Auth.JWT.Issuer)
	require.Equal(t, "aegiscore-users", cfg.Auth.JWT.Audience)
	require.Equal(t, 15*time.Minute, cfg.Auth.JWT.AccessTokenTTL)
	require.Equal(t, 168*time.Hour, cfg.Auth.JWT.RefreshTokenTTL)
	require.Equal(t, 5*time.Minute, cfg.Auth.JWT.PasswordChangeTokenTTL)
	require.Equal(t, 30*time.Second, cfg.Auth.TokenVersionCacheTTL)
	require.True(t, cfg.Auth.RefreshTokenRotation)
	require.Equal(t, 5, cfg.Auth.MaxActiveSessionsPerUser)
	require.Equal(t, 2, cfg.Auth.PasswordKDF.Argon2Concurrency)
	require.Equal(t, 16, cfg.Auth.PasswordKDF.Argon2QueueSize)
	authTokenVersion := requireLocalCacheInstance(t, cfg, "auth_token_version")
	require.Equal(t, int64(1000), authTokenVersion.Capacity)
	require.Equal(t, time.Second, authTokenVersion.TTL)
	require.Equal(t, 300*time.Millisecond, authTokenVersion.LoadTimeout)
	require.Equal(t, int64(2000), authTokenVersion.NumCounters)
	require.Equal(t, int64(128), authTokenVersion.BufferItems)
	rbacUserRoles := requireLocalCacheInstance(t, cfg, "rbac_user_roles")
	require.Equal(t, int64(2000), rbacUserRoles.Capacity)
	require.Equal(t, 5*time.Second, rbacUserRoles.TTL)
	require.Equal(t, 500*time.Millisecond, rbacUserRoles.LoadTimeout)
	backgroundJobs := requireLocalCacheInstance(t, cfg, "background_jobs")
	require.Equal(t, int64(300), backgroundJobs.Capacity)
	require.Equal(t, 10*time.Second, backgroundJobs.TTL)
	require.Equal(t, 200*time.Millisecond, backgroundJobs.LoadTimeout)
	require.True(t, cfg.Ent.SQLDebug)
	require.Equal(t, "./logs", cfg.Log.Directory)
	require.Equal(t, "aegiscore-test", cfg.Log.Filename)
	require.True(t, cfg.Log.Console)
	require.Equal(t, 7, cfg.Log.MaxAgeDays)
	require.Equal(t, 100, cfg.Log.MaxSizeMB)
	require.Equal(t, 30, cfg.Log.MaxBackups)
	require.True(t, cfg.Observability.Metrics.Enabled)
	require.Equal(t, "/metrics", cfg.Observability.Metrics.Path)
	require.True(t, cfg.Observability.Metrics.IncludeRuntime)
	require.True(t, cfg.Observability.Tracing.Enabled)
	require.Equal(t, 0.25, cfg.Observability.Tracing.SampleRatio)
	require.Equal(t, "otlp", cfg.Observability.Tracing.Exporter)
	require.Equal(t, "collector:4317", cfg.Observability.Tracing.OTLPEndpoint)
	require.False(t, cfg.Observability.Tracing.Insecure)
	cacheRedis, ok := cfg.RedisConfig("cache_redis")
	require.True(t, ok)
	require.Equal(t, 2, cacheRedis.DB)
	require.Equal(t, 7*time.Second, cacheRedis.PingTimeout)
	queueRedis, ok := cfg.RedisConfig("queue_redis")
	require.True(t, ok)
	require.Equal(t, 1, queueRedis.DB)

	pg := cfg.Postgres["user_db"]
	require.Equal(t, "127.0.0.1", pg.Host)
	require.Equal(t, 15432, pg.Port)
	require.Equal(t, "pgx", pg.Driver)
	require.Equal(t, "disable", pg.SSLMode)
	require.Equal(t, 20, pg.MaxOpenConns)
	require.Equal(t, 4, pg.MaxIdleConns)
	require.Equal(t, 45*time.Minute, pg.ConnMaxLifetime)
	require.Equal(t, 12*time.Minute, pg.ConnMaxIdleTime)
	require.Equal(t, 7*time.Second, pg.PingTimeout)
	require.Equal(t, "aegiscore_user", pg.DBName)
	require.Equal(t, "aegiscore_pay", cfg.Postgres["pay_db"].DBName)
}

func requireLocalCacheInstance(t *testing.T, cfg *Config, name string) LocalCacheInstanceConfig {
	t.Helper()
	cacheCfg, ok := cfg.LocalCache.Instance(name)
	require.True(t, ok)
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
		"auth.password_kdf.argon2_concurrency must be > 0",
		"auth.password_kdf.argon2_queue_size must be > 0",
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

func TestLoadAllowsNonPositiveTokenVersionCacheTTL(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
		want time.Duration
	}{
		{name: "zero", yaml: "0s", want: 0},
		{name: "negative", yaml: "-1s", want: -time.Second},
		{name: "positive", yaml: "30s", want: 30 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := loadConfigFromYAML(t, configYAMLWithSection(`auth:
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
  token_version_cache_ttl: `+tc.yaml+`
  refresh_token_rotation: true
  max_active_sessions_per_user: 5`))
			require.Equal(t, tc.want, cfg.Auth.TokenVersionCacheTTL)
		})
	}
}

func TestLoadStillRejectsNonPositiveJWTTTL(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`auth:
  jwt:
    secret: test-secret
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 0s
    refresh_token_ttl: -1s
  password_kdf:
    argon2_concurrency: 2
    argon2_queue_size: 16
  token_version_cache_ttl: 0s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5`))

	assertConfigLoadErrorContains(t, err,
		"auth.jwt.access_token_ttl must be > 0",
		"auth.jwt.refresh_token_ttl must be > 0",
	)
	require.NotContains(t, err.Error(), "auth.token_version_cache_ttl")
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
	require.ErrorAs(t, err, &validationErr)
	require.Len(t, validationErr.Unwrap(), 1)
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
	require.NotContains(t, err.Error(), "test-secret")
	require.NotContains(t, err.Error(), "collector.internal:4317")
}

func TestLoadEnvironmentOverride(t *testing.T) {
	t.Setenv("AEGISCORE_SYSTEM_TIMEZONE", "UTC")
	t.Setenv("AEGISCORE_HTTP_PORT", "19090")
	t.Setenv("AEGISCORE_HTTP_READ_TIMEOUT", "30s")
	t.Setenv("AEGISCORE_HTTP_WRITE_TIMEOUT", "60s")
	t.Setenv("AEGISCORE_HTTP_IDLE_TIMEOUT", "120s")
	t.Setenv("AEGISCORE_HTTP_SHUTDOWN_TIMEOUT", "25s")
	t.Setenv("AEGISCORE_HTTP_TRUSTED_PROXIES", "10.0.0.1,10.0.0.2")
	t.Setenv("AEGISCORE_AUTH_JWT_SECRET", "env-secret")
	t.Setenv("AEGISCORE_AUTH_JWT_ISSUER", "env-issuer")
	t.Setenv("AEGISCORE_AUTH_JWT_REFRESH_TOKEN_TTL", "720h")
	t.Setenv("AEGISCORE_AUTH_JWT_PASSWORD_CHANGE_TOKEN_TTL", "4m")
	t.Setenv("AEGISCORE_AUTH_PASSWORD_KDF_ARGON2_CONCURRENCY", "3")
	t.Setenv("AEGISCORE_AUTH_PASSWORD_KDF_ARGON2_QUEUE_SIZE", "9")
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
	require.Equal(t, "UTC", cfg.System.Timezone)
	require.Equal(t, 19090, cfg.HTTP.Port)
	require.Equal(t, 30*time.Second, cfg.HTTP.ReadTimeout)
	require.Equal(t, 60*time.Second, cfg.HTTP.WriteTimeout)
	require.Equal(t, 120*time.Second, cfg.HTTP.IdleTimeout)
	require.Equal(t, 25*time.Second, cfg.HTTP.ShutdownTimeout)
	require.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, cfg.HTTP.TrustedProxies)
	require.Equal(t, "env-secret", cfg.Auth.JWT.Secret)
	require.Equal(t, "env-issuer", cfg.Auth.JWT.Issuer)
	require.Equal(t, 720*time.Hour, cfg.Auth.JWT.RefreshTokenTTL)
	require.Equal(t, 4*time.Minute, cfg.Auth.JWT.PasswordChangeTokenTTL)
	require.Equal(t, 30*time.Second, cfg.Auth.TokenVersionCacheTTL)
	require.Equal(t, 7, cfg.Auth.MaxActiveSessionsPerUser)
	require.Equal(t, 3, cfg.Auth.PasswordKDF.Argon2Concurrency)
	require.Equal(t, 9, cfg.Auth.PasswordKDF.Argon2QueueSize)
	authTokenVersion := requireLocalCacheInstance(t, cfg, "auth_token_version")
	require.Equal(t, int64(3000), authTokenVersion.Capacity)
	require.Equal(t, 2*time.Second, authTokenVersion.TTL)
	require.Equal(t, 400*time.Millisecond, authTokenVersion.LoadTimeout)
	rbacUserRoles := requireLocalCacheInstance(t, cfg, "rbac_user_roles")
	require.Equal(t, int64(4000), rbacUserRoles.Capacity)
	require.Equal(t, int64(256), rbacUserRoles.BufferItems)
	backgroundJobs := requireLocalCacheInstance(t, cfg, "background_jobs")
	require.Equal(t, 15*time.Second, backgroundJobs.TTL)
	require.False(t, cfg.Ent.SQLDebug)
	require.False(t, cfg.Observability.Metrics.Enabled)
	require.Equal(t, "/internal/metrics", cfg.Observability.Metrics.Path)
	require.False(t, cfg.Observability.Metrics.IncludeRuntime)
	require.False(t, cfg.Observability.Tracing.Enabled)
	require.Equal(t, 0.5, cfg.Observability.Tracing.SampleRatio)
	require.Equal(t, "none", cfg.Observability.Tracing.Exporter)
	require.Equal(t, "env-collector:4317", cfg.Observability.Tracing.OTLPEndpoint)
	require.True(t, cfg.Observability.Tracing.Insecure)
	require.Equal(t, 9, cfg.Redis["cache_redis"].DB)
	require.Equal(t, "env-secret", cfg.Postgres["user_db"].Password)
	require.Equal(t, 30, cfg.Postgres["user_db"].MaxOpenConns)
}

func TestLoadAllowsOmittedOptionalConfigFields(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSection(`http:
  host: 127.0.0.1
  port: 18080
  read_timeout: 10s
  write_timeout: 10s
  idle_timeout: 60s
  shutdown_timeout: 10s`))
	require.Empty(t, cfg.HTTP.TrustedProxies)

	cfg = loadConfigFromYAML(t, configYAMLWithSection(`redis:
  cache_redis:
    addr: 127.0.0.1:6379
    db: 2
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    ping_timeout: 7s`))
	require.Empty(t, cfg.Redis["cache_redis"].Username)
	require.Empty(t, cfg.Redis["cache_redis"].Password)

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
	require.Empty(t, cfg.Postgres["user_db"].Password)
	_, ok := cfg.PostgresDatabaseConfig("pay_db")
	require.False(t, ok)
}

func TestLoadValidatesPasswordKDFConfig(t *testing.T) {
	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`auth:
  jwt:
    secret: test-secret
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 15m
    refresh_token_ttl: 168h
    password_change_token_ttl: 5m
  password_kdf:
    argon2_concurrency: 0
    argon2_queue_size: -1
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5`))
	assertConfigLoadErrorContains(t, err,
		"auth.password_kdf.argon2_concurrency must be > 0",
		"auth.password_kdf.argon2_queue_size must be > 0",
	)

	err = loadConfigErrorFromYAML(t, configYAMLWithSection(`auth:
  jwt:
    secret: test-secret
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 15m
    refresh_token_ttl: 168h
    password_change_token_ttl: 5m
  password_kdf:
    argon2_concurrency: 3
    argon2_queue_size: 2
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5`))
	assertConfigLoadErrorContains(t, err, "auth.password_kdf.argon2_queue_size must be >= auth.password_kdf.argon2_concurrency")
}

func TestLoadValidatesAuthSessionLimit(t *testing.T) {
	cfg := loadConfigFromYAML(t, configYAMLWithSection(`auth:
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
  max_active_sessions_per_user: 0`))
	require.Equal(t, 0, cfg.Auth.MaxActiveSessionsPerUser)

	err := loadConfigErrorFromYAML(t, configYAMLWithSection(`auth:
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

	require.Len(t, err, 1)
	require.EqualError(t, err[0], "local_cache must not contain an empty named instance")
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

	require.Equal(t, 10*time.Second, cfg.Redis["queue_redis"].DialTimeout)
	require.Equal(t, 3*time.Second, cfg.Redis["queue_redis"].ReadTimeout)
	require.Equal(t, 20, cfg.Postgres["user_db"].MaxOpenConns)
	require.Equal(t, 25, cfg.Postgres["pay_db"].MaxOpenConns)
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
			require.True(t, ok)
			parsed, err := url.Parse(db.DSN)
			require.NoError(t, err)
			require.Equal(t, "postgres", parsed.Scheme)
			require.Equal(t, "db.example.internal:15432", parsed.Host)
			require.Equal(t, tt.wantDBName, strings.TrimPrefix(parsed.Path, "/"))
			require.Equal(t, "user@example.com", parsed.User.Username())
			password, ok := parsed.User.Password()
			require.True(t, ok)
			require.Equal(t, "p@ss/w:rd", password)
			require.Equal(t, "disable", parsed.Query().Get("sslmode"))
		})
	}

	_, ok := cfg.PostgresDatabaseConfig("pay_db")
	require.True(t, ok)
	_, ok = cfg.PostgresDatabaseConfig("missing_db")
	require.False(t, ok)
}

func TestRedisConfigLookup(t *testing.T) {
	cfg := loadConfigFromYAML(t, explicitConfigYAML())
	redisCfg, ok := cfg.RedisConfig("cache_redis")
	require.True(t, ok)
	require.Equal(t, "127.0.0.1:6379", redisCfg.Addr)
	_, ok = cfg.RedisConfig("missing_redis")
	require.False(t, ok)
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
    password_change_token_ttl: 5m
  password_kdf:
    argon2_concurrency: 2
    argon2_queue_size: 16
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
    password_change_token_ttl: 5m
  password_kdf:
    argon2_concurrency: 2
    argon2_queue_size: 16
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
	require.NoError(t, err)
	return cfg
}

func loadConfigErrorFromYAML(t *testing.T, content string) error {
	t.Helper()
	_, err := Load(writeTempConfig(t, content))
	require.Error(t, err)
	return err
}

func assertConfigLoadErrorContains(t *testing.T, err error, wants ...string) {
	t.Helper()
	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	for _, want := range wants {
		require.Contains(t, err.Error(), want)
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
