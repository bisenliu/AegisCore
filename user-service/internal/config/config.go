package config

import (
	"net/http"
	"strings"
	"time"

	platformmw "github.com/aegiscore/platform/http/middleware"
	platformconfig "github.com/aegiscore/platform/runtime/config"
	"github.com/aegiscore/platform/runtime/localcache"
	platformresources "github.com/aegiscore/platform/runtime/resources"
	serviceresources "github.com/aegiscore/user-service/internal/resources"
)

const (
	// minProductionJWTBytes 是生产环境中 JWT 对称密钥的最小合法字节长度（256 位）。
	minProductionJWTBytes = 32
	// DefaultHTTPRequestBodyMaxBytes 是 user-service JSON 请求体的默认字节上限。
	DefaultHTTPRequestBodyMaxBytes int64 = 64 * 1024
	// DefaultEntSlowQueryThreshold 是 Ent SQL log 插件的慢查询阈值。
	DefaultEntSlowQueryThreshold = 500 * time.Millisecond
	// DefaultEntSQLLogDebug 控制 SQL log 插件是否记录成功 SQL。
	DefaultEntSQLLogDebug = false

	// PolicyWatcherSubscribeTimeout 是 policy watcher 等待 Redis 订阅确认的预算。
	PolicyWatcherSubscribeTimeout = 5 * time.Second
	// PolicyWatcherRetryBackoffInitial 是 policy watcher 重连退避的初始值。
	PolicyWatcherRetryBackoffInitial = 250 * time.Millisecond
	// PolicyWatcherRetryBackoffMax 是 policy watcher 重连退避的上限。
	PolicyWatcherRetryBackoffMax = 30 * time.Second

	// AnonymousRateLimitShards 是匿名限流器的锁分片数。
	AnonymousRateLimitShards = 64
	// AuthenticatedRateLimitShards 是已认证限流器的锁分片数。
	AuthenticatedRateLimitShards = 128
	// RateLimitCleanupInterval 是限流 key janitor 的清理周期。
	RateLimitCleanupInterval = 30 * time.Second
	// RateLimitSlotsPerShard 是限流器每个分片默认分配的容量槽位数。
	RateLimitSlotsPerShard = 1024
	// DefaultAPIRateLimitWindow 是默认 API 限流统计的时间窗口时长。
	DefaultAPIRateLimitWindow = 10 * time.Minute

	// DefaultPolicyWatcherCheckInterval 是策略监听器主动轮询数据库版本的默认周期。
	DefaultPolicyWatcherCheckInterval = 15 * time.Second
	// DefaultPolicyWatcherMaxStaleness 是策略监听器判定校准失效的默认最大陈旧时长。
	DefaultPolicyWatcherMaxStaleness = 45 * time.Second
	// DefaultUserRoleCacheTTL 是用户角色映射本地缓存的默认过期时长。
	DefaultUserRoleCacheTTL = 5 * time.Second
	// DefaultTokenVersionCacheTTL 是令牌版本号本地缓存的默认过期时长。
	DefaultTokenVersionCacheTTL = time.Second
	// DefaultLoginLockoutMaxFailures 是同一用户名连续密码失败后锁定的默认次数。
	DefaultLoginLockoutMaxFailures = 5
	// DefaultLoginLockoutFailureWindow 是连续密码失败计数的默认观察窗口。
	DefaultLoginLockoutFailureWindow = 15 * time.Minute
	// DefaultLoginLockoutTTL 是达到失败阈值后拒绝该用户名登录的默认时长。
	DefaultLoginLockoutTTL = 15 * time.Minute
)

var sensitiveConfigPaths = []string{
	// JWT secret 是 auth feature 私有签名材料，platform 只负责执行调用方声明的脱敏路径。
	"auth.jwt.secret",
	// Redis/PostgreSQL 具名资源属于 user-service 运行时接线策略，由服务决定哪些字段敏感。
	"resources.redis.*.password",
	"resources.postgres.*.password",
}

// Config 是 user-service 的根配置对象。
type Config struct {
	platformconfig.Config `mapstructure:",squash"`
	Resources             ResourcesConfig    `mapstructure:"resources"`
	Auth                  AuthConfig         `mapstructure:"auth"`
	RBAC                  RBACConfig         `mapstructure:"rbac"`
	APIRateLimit          APIRateLimitConfig `mapstructure:"api_rate_limit"`
	HTTP                  HTTPConfig         `mapstructure:"http"`
	Ent                   EntConfig          `mapstructure:"ent"`
}

// HTTPConfig 包含 user-service 私有 HTTP 暴露策略。
type HTTPConfig struct {
	RequestBodyMaxBytes int64      `mapstructure:"request_body_max_bytes"`
	CORS                CORSConfig `mapstructure:"cors"`
}

// CORSConfig 定义 HTTP 跨域访问控制策略配置。
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAgeSeconds    int      `mapstructure:"max_age_seconds"`
}

// DefaultCORSConfig 返回安全收敛的默认跨域配置。
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		Enabled: false,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
		},
		MaxAgeSeconds: 43200,
	}
}

// Validate 校验跨域策略配置。若启用则要求来源不可为空，且禁止在凭据模式下全开通配符。
func (c CORSConfig) Validate(path string) []error {
	if !c.Enabled {
		return nil
	}
	var errs []error
	if len(c.AllowedOrigins) == 0 {
		errs = append(errs, platformconfig.FieldError(path+".allowed_origins", "must not be empty when cors is enabled"))
	}
	for _, origin := range c.AllowedOrigins {
		if origin == "*" && c.AllowCredentials {
			errs = append(errs, platformconfig.FieldError(path+".allow_credentials", "cannot be true when allowed_origins contains '*'"))
		}
	}
	return errs
}

// ToOptions 将配置转换为中间件所需的运行时选项。
func (c CORSConfig) ToOptions() platformmw.CORSOptions {
	methods := c.AllowedMethods
	if len(methods) == 0 {
		methods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	}
	headers := c.AllowedHeaders
	if len(headers) == 0 {
		headers = []string{"Authorization", "Content-Type"}
	}
	return platformmw.CORSOptions{
		AllowedOrigins:   c.AllowedOrigins,
		AllowedMethods:   methods,
		AllowedHeaders:   headers,
		ExposedHeaders:   c.ExposedHeaders,
		AllowCredentials: c.AllowCredentials,
		MaxAgeSeconds:    c.MaxAgeSeconds,
	}
}

// ResourcesConfig 包含 user-service 按名称声明的外部运行时资源。
type ResourcesConfig struct {
	Redis    platformresources.RedisConfigs    `mapstructure:"redis"`
	Postgres platformresources.PostgresConfigs `mapstructure:"postgres"`
}

// AuthConfig 包含 user-service 认证 token 与会话校验设置。
// TokenVersionCache 由各服务副本独立持有，其 TTL 表示正常路径可接受的跨副本撤销收敛窗口。
type AuthConfig struct {
	JWT                      JWTConfig          `mapstructure:"jwt"`
	TokenVersionCache        FeatureCacheConfig `mapstructure:"token_version_cache"`
	TokenVersionCacheTTL     time.Duration      `mapstructure:"token_version_cache_ttl"`
	RefreshTokenRotation     bool               `mapstructure:"refresh_token_rotation"`
	MaxActiveSessionsPerUser int                `mapstructure:"max_active_sessions_per_user"`
	LoginLockout             LoginLockoutConfig `mapstructure:"login_lockout"`
}

// LoginLockoutConfig 控制同一用户名连续密码失败后的临时锁定。
// 它按用户名计数，不替代按客户端 IP 的匿名限流。
type LoginLockoutConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	MaxFailures   int           `mapstructure:"max_failures"`
	FailureWindow time.Duration `mapstructure:"failure_window"`
	LockoutTTL    time.Duration `mapstructure:"lockout_ttl"`
}

// DefaultLoginLockoutConfig 返回启用状态下的登录锁定默认配置。
func DefaultLoginLockoutConfig() LoginLockoutConfig {
	return LoginLockoutConfig{
		Enabled:       true,
		MaxFailures:   DefaultLoginLockoutMaxFailures,
		FailureWindow: DefaultLoginLockoutFailureWindow,
		LockoutTTL:    DefaultLoginLockoutTTL,
	}
}

// Validate 校验启用的登录锁定策略；禁用时其余字段不参与运行时判定。
func (c LoginLockoutConfig) Validate(path string) []error {
	if !c.Enabled {
		return nil
	}
	var errs []error
	if c.MaxFailures <= 0 {
		errs = append(errs, platformconfig.FieldError(path+".max_failures", "must be > 0 when enabled"))
	}
	errs = append(errs, platformconfig.ValidatePositiveDuration(path+".failure_window", c.FailureWindow)...)
	errs = append(errs, platformconfig.ValidatePositiveDuration(path+".lockout_ttl", c.LockoutTTL)...)
	return errs
}

// RBACConfig 包含 user-service RBAC 热路径的服务私有设置。
type RBACConfig struct {
	UserRoleCache FeatureCacheConfig  `mapstructure:"user_role_cache"`
	PolicyWatcher PolicyWatcherConfig `mapstructure:"policy_watcher"`
}

// PolicyWatcherConfig 控制 RBAC policy watcher 的校准周期和新鲜度预算。
// 订阅确认超时和重连退避由代码常量拥有，不进入配置文件。
type PolicyWatcherConfig struct {
	CheckInterval time.Duration `mapstructure:"check_interval"`
	MaxStaleness  time.Duration `mapstructure:"max_staleness"`
}

// DefaultPolicyWatcherConfig 返回 RBAC policy watcher 的外部配置默认值。
func DefaultPolicyWatcherConfig() PolicyWatcherConfig {
	return PolicyWatcherConfig{
		CheckInterval: DefaultPolicyWatcherCheckInterval,
		MaxStaleness:  DefaultPolicyWatcherMaxStaleness,
	}
}

// Validate 校验 policy watcher 的校准周期和新鲜度预算。
func (c PolicyWatcherConfig) Validate(path string) []error {
	var errs []error
	errs = append(errs, platformconfig.ValidatePositiveDuration(path+".check_interval", c.CheckInterval)...)
	errs = append(errs, platformconfig.ValidatePositiveDuration(path+".max_staleness", c.MaxStaleness)...)
	if c.MaxStaleness > 0 && c.CheckInterval > 0 && c.MaxStaleness <= c.CheckInterval {
		errs = append(errs, platformconfig.FieldError(path+".max_staleness", "must be > check_interval"))
	}
	return errs
}

// APIRateLimitConfig 包含 user-service API 限流的服务私有配置。
type APIRateLimitConfig struct {
	Anonymous     RateLimitPolicyConfig `mapstructure:"anonymous"`
	Authenticated RateLimitPolicyConfig `mapstructure:"authenticated"`
}

// RateLimitPolicyConfig 描述一个限流策略的外部配置面。
// 分片数量和清理周期由代码常量拥有，不进入配置文件。
type RateLimitPolicyConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	RatePerSecond  float64       `mapstructure:"rate_per_second"`
	Burst          int           `mapstructure:"burst"`
	MaxKeys        int           `mapstructure:"max_keys"`
	CapacityPolicy string        `mapstructure:"capacity_policy"`
	KeyTTL         time.Duration `mapstructure:"key_ttl"`
}

// DefaultRateLimitPolicyConfig 构造启用状态下的限流策略默认配置。
func DefaultRateLimitPolicyConfig(ratePerSecond float64, burst int, keyTTL time.Duration, maxKeys int) RateLimitPolicyConfig {
	return RateLimitPolicyConfig{
		Enabled:        true,
		RatePerSecond:  ratePerSecond,
		Burst:          burst,
		MaxKeys:        maxKeys,
		CapacityPolicy: string(platformmw.RateLimitCapacityPolicyOverflow),
		KeyTTL:         keyTTL,
	}
}

// Validate 校验启用的限流策略；禁用时其余字段不参与运行时构造和校验。
func (c RateLimitPolicyConfig) Validate(path string) []error {
	if !c.Enabled {
		return nil
	}
	var errs []error
	if c.RatePerSecond <= 0 {
		errs = append(errs, platformconfig.FieldError(path+".rate_per_second", "must be > 0 when enabled"))
	}
	if c.Burst <= 0 {
		errs = append(errs, platformconfig.FieldError(path+".burst", "must be > 0 when enabled"))
	}
	if c.MaxKeys <= 0 {
		errs = append(errs, platformconfig.FieldError(path+".max_keys", "must be > 0 when enabled"))
	}
	switch platformmw.RateLimitCapacityPolicy(strings.TrimSpace(c.CapacityPolicy)) {
	case platformmw.RateLimitCapacityPolicyOverflow, platformmw.RateLimitCapacityPolicyReject:
	case "":
		errs = append(errs, platformconfig.FieldError(path+".capacity_policy", "is required when enabled"))
	default:
		errs = append(errs, platformconfig.FieldError(path+".capacity_policy", "must be one of overflow or reject when enabled"))
	}
	errs = append(errs, platformconfig.ValidatePositiveDuration(path+".key_ttl", c.KeyTTL)...)
	return errs
}

const (
	// TokenVersionCacheCapacity 是 token version 本地缓存的最大 item 数。
	TokenVersionCacheCapacity uint64 = 100000
	// TokenVersionCacheLoadTimeout 是 token version 单次回源预算。
	TokenVersionCacheLoadTimeout = 300 * time.Millisecond
	// UserRoleCacheCapacity 是用户角色本地缓存的最大 item 数。
	UserRoleCacheCapacity uint64 = 100000
	// UserRoleCacheLoadTimeout 是用户角色单次回源预算。
	UserRoleCacheLoadTimeout = 500 * time.Millisecond
)

// FeatureCacheConfig 描述单个 feature 自有 bounded cache 的外部配置面。
// 容量和回源超时由代码常量拥有，不进入配置文件。
type FeatureCacheConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	TTL     time.Duration `mapstructure:"ttl"`
}

// DefaultFeatureCacheConfig 构造启用状态下的 feature cache 默认配置。
func DefaultFeatureCacheConfig(ttl time.Duration) FeatureCacheConfig {
	return FeatureCacheConfig{
		Enabled: true,
		TTL:     ttl,
	}
}

// Validate 校验启用的 feature cache；禁用时 TTL 不参与运行时构造和校验。
func (c FeatureCacheConfig) Validate(path string) []error {
	if !c.Enabled {
		return nil
	}
	return platformconfig.ValidatePositiveDuration(path+".ttl", c.TTL)
}

// Localcache 将服务私有 feature cache 配置集中映射为通用 localcache 配置。
// capacity 与 loadTimeout 必须由调用方传入该 feature 的代码常量。
func (c FeatureCacheConfig) Localcache(name string, capacity uint64, loadTimeout time.Duration) localcache.Config {
	return localcache.Config{
		Name:        name,
		Capacity:    capacity,
		TTL:         c.TTL,
		LoadTimeout: loadTimeout,
	}
}

// EntConfig 控制 user-service Ent 运行时行为。
type EntConfig struct {
	Plugins EntPluginsConfig `mapstructure:"plugins"`
}

// EntPluginsConfig 控制 Ent 运行时观测插件启停。
type EntPluginsConfig struct {
	SQLLog  EntSQLLogPluginConfig  `mapstructure:"sql_log"`
	Tracing EntTracingPluginConfig `mapstructure:"tracing"`
	Metrics EntMetricsPluginConfig `mapstructure:"metrics"`
}

// EntSQLLogPluginConfig 控制 Ent SQL driver 日志插件启停。
// debug 输出和慢查询阈值由代码常量拥有，不进入配置文件。
type EntSQLLogPluginConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// EntTracingPluginConfig 控制 Ent tracing 插件行为。
type EntTracingPluginConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// EntMetricsPluginConfig 控制 Ent Prometheus metrics 插件行为。
type EntMetricsPluginConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// JWTConfig 包含 user-service JWT 签发和校验设置。
type JWTConfig struct {
	Secret                 string        `mapstructure:"secret"`
	Issuer                 string        `mapstructure:"issuer"`
	Audience               string        `mapstructure:"audience"`
	AccessTokenTTL         time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL        time.Duration `mapstructure:"refresh_token_ttl"`
	PasswordChangeTokenTTL time.Duration `mapstructure:"password_change_token_ttl"`
}

// RuntimeConfig 返回共享 runtime 配置副本。
func (c Config) RuntimeConfig() platformconfig.Config {
	return c.Config
}

// SensitiveConfigPaths 返回 user-service 拥有的配置脱敏路径副本。
//
// 返回副本是为了避免测试或未来调用方修改 slice 后影响 CLI render 的安全策略。
func SensitiveConfigPaths() []string {
	return append([]string(nil), sensitiveConfigPaths...)
}

// RedactEffectiveSettings 使用 user-service 敏感路径策略返回脱敏副本。
//
// 该函数是 user-service 配置渲染边界的唯一入口：服务在这里集中声明自身 secret schema，
// platform/runtime/config 只执行业务中立的路径匹配和深拷贝。
func RedactEffectiveSettings(settings map[string]any) map[string]any {
	return platformconfig.RedactSettings(settings, sensitiveConfigPaths)
}

// DefaultConfig 返回 user-service 的完整默认配置初值。
func DefaultConfig() Config {
	return Config{
		Config: platformconfig.DefaultConfig(),
		Resources: ResourcesConfig{
			Redis: platformresources.RedisConfigs{
				serviceresources.NameCacheRedis: {
					Mode:    platformresources.RedisModeCluster,
					Timeout: platformresources.DefaultRedisTimeout,
					Cluster: platformresources.RedisClusterConfig{
						MaxRedirects: platformresources.DefaultRedisClusterMaxRedirects,
					},
				},
			},
			Postgres: platformresources.PostgresConfigs{
				serviceresources.NamePrimaryDB: {
					SSLMode: platformresources.DefaultPostgresSSLMode,
					Pool: platformresources.PostgresPoolConfig{
						MaxOpenConns:    platformresources.DefaultPostgresMaxOpenConns,
						MaxIdleConns:    platformresources.DefaultPostgresMaxIdleConns,
						ConnMaxLifetime: platformresources.DefaultPostgresConnMaxLifetime,
						ConnMaxIdleTime: platformresources.DefaultPostgresConnMaxIdleTime,
					},
				},
			},
		},
		Auth: AuthConfig{
			// 默认接受 logout-all 和改密在正常 Redis 更新成功后最多 1 秒的跨副本撤销收敛窗口。
			TokenVersionCache: DefaultFeatureCacheConfig(DefaultTokenVersionCacheTTL),
			LoginLockout:      DefaultLoginLockoutConfig(),
		},
		RBAC: RBACConfig{
			UserRoleCache: DefaultFeatureCacheConfig(DefaultUserRoleCacheTTL),
			PolicyWatcher: DefaultPolicyWatcherConfig(),
		},
		APIRateLimit: APIRateLimitConfig{
			Anonymous:     DefaultRateLimitPolicyConfig(1, 5, DefaultAPIRateLimitWindow, AnonymousRateLimitShards*RateLimitSlotsPerShard),
			Authenticated: DefaultRateLimitPolicyConfig(5, 20, DefaultAPIRateLimitWindow, AuthenticatedRateLimitShards*RateLimitSlotsPerShard),
		},
		HTTP: HTTPConfig{
			RequestBodyMaxBytes: DefaultHTTPRequestBodyMaxBytes,
			CORS:                DefaultCORSConfig(),
		},
		Ent: EntConfig{
			Plugins: EntPluginsConfig{
				Tracing: EntTracingPluginConfig{Enabled: true},
			},
		},
	}
}

// normalizeConfig 按 raw settings 保留实际声明的固定资源，并补齐新增动态具名资源的通用默认值。
func normalizeConfig(c *Config, settings map[string]any) {
	if c == nil {
		return
	}
	if !hasRawNamedResource(settings, "redis", serviceresources.NameCacheRedis) {
		delete(c.Resources.Redis, serviceresources.NameCacheRedis)
	}
	if !hasRawNamedResource(settings, "postgres", serviceresources.NamePrimaryDB) {
		delete(c.Resources.Postgres, serviceresources.NamePrimaryDB)
	}
	c.Resources.Redis.ApplyDefaults()
	c.Resources.Postgres.ApplyDefaults()
}

// hasRawNamedResource 判断 raw settings 中是否显式声明了某类具名资源。
// normalize 阶段依赖该判断删除未声明的默认固定资源，避免默认值伪造资源存在性。
func hasRawNamedResource(settings map[string]any, resourceType string, name string) bool {
	resources, ok := settings["resources"].(map[string]any)
	if !ok {
		return false
	}
	namedResources, ok := resources[resourceType].(map[string]any)
	if !ok {
		return false
	}
	value, exists := namedResources[name]
	return exists && value != nil
}

// Validate 在 user-service 启动前拒绝结构非法的服务配置。
// 先复用 platform runtime 校验，再追加 user-service 认证约束；返回聚合错误，便于一次性展示所有字段问题。
func (c Config) Validate() error {
	var errs []error
	if err := c.Config.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Resources.Redis.Validate("resources.redis"); err != nil {
		errs = append(errs, err)
	}
	if err := c.Resources.Postgres.Validate("resources.postgres"); err != nil {
		errs = append(errs, err)
	}
	if _, ok := c.Resources.Redis[serviceresources.NameCacheRedis]; !ok {
		errs = append(errs, platformconfig.FieldError("resources.redis."+serviceresources.NameCacheRedis, "is required"))
	}
	if _, ok := c.Resources.Postgres[serviceresources.NamePrimaryDB]; !ok {
		errs = append(errs, platformconfig.FieldError("resources.postgres."+serviceresources.NamePrimaryDB, "is required"))
	}
	errs = append(errs, c.validateAuth()...)
	errs = append(errs, c.RBAC.UserRoleCache.Validate("rbac.user_role_cache")...)
	errs = append(errs, c.RBAC.PolicyWatcher.Validate("rbac.policy_watcher")...)
	errs = append(errs, c.APIRateLimit.Anonymous.Validate("api_rate_limit.anonymous")...)
	errs = append(errs, c.APIRateLimit.Authenticated.Validate("api_rate_limit.authenticated")...)
	if c.HTTP.RequestBodyMaxBytes <= 0 {
		errs = append(errs, platformconfig.FieldError("http.request_body_max_bytes", "must be > 0"))
	}
	errs = append(errs, c.HTTP.CORS.Validate("http.cors")...)
	if len(errs) == 0 {
		return nil
	}
	return platformconfig.NewValidationError(errs)
}

// validateAuth 校验 user-service auth 私有配置，包括 JWT secret、TTL 和会话上限。
func (c Config) validateAuth() []error {
	// production-like 环境对 JWT secret 额外加固；0 个活跃会话上限表示不裁剪。
	var errs []error
	secret := strings.TrimSpace(c.Auth.JWT.Secret)
	if secret == "" {
		errs = append(errs, platformconfig.FieldError("auth.jwt.secret", "is required"))
	} else if c.isProductionLike() {
		if isInsecureJWTSecret(strings.ToLower(secret)) {
			errs = append(errs, platformconfig.FieldError("auth.jwt.secret", "must not use a development default in production-like environments"))
		} else if len([]byte(secret)) < minProductionJWTBytes {
			errs = append(errs, platformconfig.FieldError("auth.jwt.secret", "must be at least 32 bytes in production-like environments"))
		}
	}
	errs = append(errs, platformconfig.ValidatePositiveDuration("auth.jwt.access_token_ttl", c.Auth.JWT.AccessTokenTTL)...)
	errs = append(errs, platformconfig.ValidatePositiveDuration("auth.jwt.refresh_token_ttl", c.Auth.JWT.RefreshTokenTTL)...)
	errs = append(errs, platformconfig.ValidateNonNegativeInt("auth.max_active_sessions_per_user", c.Auth.MaxActiveSessionsPerUser)...)
	errs = append(errs, c.Auth.TokenVersionCache.Validate("auth.token_version_cache")...)
	errs = append(errs, c.Auth.LoginLockout.Validate("auth.login_lockout")...)
	return errs
}

// isProductionLike 判断 user-service 环境是否需要生产级 secret 校验。
func (c Config) isProductionLike() bool {
	// staging 按生产级安全策略校验，避免预发环境使用开发默认 secret。
	switch strings.ToLower(strings.TrimSpace(c.App.Environment)) {
	case "prod", "production", "staging":
		return true
	default:
		return false
	}
}

// isInsecureJWTSecret 判断 JWT secret 是否属于禁止用于生产类环境的开发默认值。
func isInsecureJWTSecret(value string) bool {
	switch value {
	case "changeme", "local-development-secret", "secret", "test-secret":
		return true
	default:
		return false
	}
}
