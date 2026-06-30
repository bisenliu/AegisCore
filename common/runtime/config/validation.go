package config

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

const (
	minPort = 1
	maxPort = 65535
)

var (
	validLogLevels = map[string]struct{}{
		"debug": {},
		"info":  {},
		"warn":  {},
		"error": {},
	}
	validLogFormats = map[string]struct{}{
		"json":    {},
		"console": {},
		"text":    {},
	}
	validPostgresDrivers = map[string]struct{}{
		"pgx": {},
	}
	validPostgresSSLModes = map[string]struct{}{
		"disable":     {},
		"allow":       {},
		"prefer":      {},
		"require":     {},
		"verify-ca":   {},
		"verify-full": {},
	}
	validTracingExporters = map[string]struct{}{
		"none": {},
		"otlp": {},
	}
	productionLikeEnvironments = map[string]struct{}{
		"prod":       {},
		"production": {},
		"staging":    {},
	}
	insecureJWTSecrets = map[string]struct{}{
		"changeme":                 {},
		"local-development-secret": {},
		"secret":                   {},
		"test-secret":              {},
	}
)

// ValidationError 聚合配置校验失败，使启动阶段能一次性报告全部非法字段。
type ValidationError struct {
	errs []error
}

func newValidationError(errs []error) *ValidationError {
	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{errs: errs}
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.errs) == 0 {
		return "config validation failed"
	}
	parts := make([]string, 0, len(e.errs))
	for _, err := range e.errs {
		parts = append(parts, err.Error())
	}
	return "config validation failed: " + strings.Join(parts, "; ")
}

func (e *ValidationError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return e.errs
}

// Validate 在服务启动前拒绝结构非法的运行时配置。
// 它只检查服务无关配置；服务特定的必需命名资源和业务策略属于服务模块。
func (c Config) Validate() error {
	var errs []error

	errs = append(errs, c.validateSystem()...)
	errs = append(errs, c.validateApp()...)
	errs = append(errs, c.validateHTTP()...)
	errs = append(errs, c.validateAuth()...)
	errs = append(errs, c.validateLog()...)
	errs = append(errs, c.validateObservability()...)
	errs = append(errs, c.validateLocalCache()...)
	errs = append(errs, c.validateRedis()...)
	errs = append(errs, c.validatePostgres()...)

	if len(errs) == 0 {
		return nil
	}
	return newValidationError(errs)
}

func (c Config) validateSystem() []error {
	var errs []error
	timezone := strings.TrimSpace(c.System.Timezone)
	if timezone == "" {
		return append(errs, configFieldError("system.timezone", "is required"))
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		errs = append(errs, configFieldError("system.timezone", "must be a valid IANA timezone"))
	}
	return errs
}

func (c Config) validateApp() []error {
	var errs []error
	if strings.TrimSpace(c.App.Name) == "" {
		errs = append(errs, configFieldError("app.name", "is required"))
	}
	if strings.TrimSpace(c.App.Environment) == "" {
		errs = append(errs, configFieldError("app.environment", "is required"))
	}
	return errs
}

func (c Config) validateHTTP() []error {
	var errs []error
	if strings.TrimSpace(c.HTTP.Host) == "" {
		errs = append(errs, configFieldError("http.host", "is required"))
	}
	errs = append(errs, validatePort("http.port", c.HTTP.Port)...)
	errs = append(errs, validatePositiveDuration("http.read_timeout", c.HTTP.ReadTimeout)...)
	errs = append(errs, validatePositiveDuration("http.write_timeout", c.HTTP.WriteTimeout)...)
	errs = append(errs, validatePositiveDuration("http.idle_timeout", c.HTTP.IdleTimeout)...)
	errs = append(errs, validatePositiveDuration("http.shutdown_timeout", c.HTTP.ShutdownTimeout)...)
	return errs
}

func (c Config) validateAuth() []error {
	var errs []error
	secret := strings.TrimSpace(c.Auth.JWT.Secret)
	if secret == "" {
		errs = append(errs, configFieldError("auth.jwt.secret", "is required"))
	} else if c.isProductionLike() {
		if _, insecure := insecureJWTSecrets[strings.ToLower(secret)]; insecure {
			errs = append(errs, configFieldError("auth.jwt.secret", "must not use a development default in production-like environments"))
		}
	}
	errs = append(errs, validatePositiveDuration("auth.jwt.access_token_ttl", c.Auth.JWT.AccessTokenTTL)...)
	errs = append(errs, validatePositiveDuration("auth.jwt.refresh_token_ttl", c.Auth.JWT.RefreshTokenTTL)...)
	errs = append(errs, validatePositiveInt("auth.password_kdf.argon2_concurrency", c.Auth.PasswordKDF.Argon2Concurrency)...)
	errs = append(errs, validatePositiveInt("auth.password_kdf.argon2_queue_size", c.Auth.PasswordKDF.Argon2QueueSize)...)
	if c.Auth.PasswordKDF.Argon2Concurrency > 0 && c.Auth.PasswordKDF.Argon2QueueSize > 0 && c.Auth.PasswordKDF.Argon2QueueSize < c.Auth.PasswordKDF.Argon2Concurrency {
		errs = append(errs, configFieldError("auth.password_kdf.argon2_queue_size", "must be >= auth.password_kdf.argon2_concurrency"))
	}
	errs = append(errs, validateNonNegativeInt("auth.max_active_sessions_per_user", c.Auth.MaxActiveSessionsPerUser)...)
	return errs
}

func (c Config) validateLog() []error {
	var errs []error
	level := strings.ToLower(strings.TrimSpace(c.Log.Level))
	if level == "" {
		errs = append(errs, configFieldError("log.level", "is required"))
	} else if _, ok := validLogLevels[level]; !ok {
		errs = append(errs, configFieldError("log.level", "must be one of debug, info, warn, error"))
	}
	format := strings.ToLower(strings.TrimSpace(c.Log.Format))
	if format == "" {
		errs = append(errs, configFieldError("log.format", "is required"))
	} else if _, ok := validLogFormats[format]; !ok {
		errs = append(errs, configFieldError("log.format", "must be one of json, console, text"))
	}
	if c.Log.Directory != "" || c.Log.Filename != "" {
		if strings.TrimSpace(c.Log.Directory) == "" {
			errs = append(errs, configFieldError("log.directory", "is required when file logging is configured"))
		}
		if strings.TrimSpace(c.Log.Filename) == "" {
			errs = append(errs, configFieldError("log.filename", "is required when file logging is configured"))
		}
	}
	errs = append(errs, validateNonNegativeInt("log.max_age_days", c.Log.MaxAgeDays)...)
	errs = append(errs, validateNonNegativeInt("log.max_size_mb", c.Log.MaxSizeMB)...)
	errs = append(errs, validateNonNegativeInt("log.max_backups", c.Log.MaxBackups)...)
	return errs
}

func (c Config) validateObservability() []error {
	var errs []error
	metricsPath := strings.TrimSpace(c.Observability.Metrics.Path)
	if metricsPath == "" {
		errs = append(errs, configFieldError("observability.metrics.path", "is required"))
	} else if c.Observability.Metrics.Enabled && !strings.HasPrefix(metricsPath, "/") {
		errs = append(errs, configFieldError("observability.metrics.path", "must start with / when metrics is enabled"))
	}
	if c.Observability.Tracing.SampleRatio < 0 || c.Observability.Tracing.SampleRatio > 1 {
		errs = append(errs, configFieldError("observability.tracing.sample_ratio", "must be between 0 and 1"))
	}
	exporter := strings.ToLower(strings.TrimSpace(c.Observability.Tracing.Exporter))
	if exporter == "" {
		errs = append(errs, configFieldError("observability.tracing.exporter", "is required"))
	} else if _, ok := validTracingExporters[exporter]; !ok {
		errs = append(errs, configFieldError("observability.tracing.exporter", "must be one of none, otlp"))
	}
	if exporter == "otlp" {
		if strings.TrimSpace(c.Observability.Tracing.OTLPEndpoint) == "" {
			errs = append(errs, configFieldError("observability.tracing.otlp_endpoint", "is required when exporter is otlp"))
		}
		if c.isProductionLike() && c.Observability.Tracing.Insecure {
			errs = append(errs, configFieldError("observability.tracing.insecure", "must not be true with otlp exporter in production-like environments"))
		}
	}
	return errs
}

func (c Config) validateLocalCache() []error {
	var errs []error
	for _, name := range sortedLocalCacheNames(c.LocalCache) {
		cacheCfg := c.LocalCache[name]
		if strings.TrimSpace(name) == "" {
			errs = append(errs, configFieldError("local_cache", "must not contain an empty named instance"))
			continue
		}
		errs = append(errs, validateLocalCacheInstance("local_cache."+name, cacheCfg)...)
	}
	return errs
}

func validateLocalCacheInstance(base string, cfg LocalCacheInstanceConfig) []error {
	var errs []error
	errs = append(errs, validatePositiveInt64(base+".capacity", cfg.Capacity)...)
	errs = append(errs, validatePositiveDuration(base+".ttl", cfg.TTL)...)
	errs = append(errs, validatePositiveDuration(base+".load_timeout", cfg.LoadTimeout)...)
	errs = append(errs, validateNonNegativeInt64(base+".num_counters", cfg.NumCounters)...)
	errs = append(errs, validateNonNegativeInt64(base+".buffer_items", cfg.BufferItems)...)
	return errs
}

func (c Config) validateRedis() []error {
	var errs []error
	for _, name := range sortedRedisNames(c.Redis) {
		redisCfg := c.Redis[name]
		base := "redis." + name
		if strings.TrimSpace(name) == "" {
			errs = append(errs, configFieldError("redis", "must not contain an empty named instance"))
		}
		if strings.TrimSpace(redisCfg.Addr) == "" {
			errs = append(errs, configFieldError(base+".addr", "is required"))
		} else {
			errs = append(errs, validateHostPort(base+".addr", redisCfg.Addr)...)
		}
		errs = append(errs, validateNonNegativeInt(base+".db", redisCfg.DB)...)
		errs = append(errs, validatePositiveDuration(base+".dial_timeout", redisCfg.DialTimeout)...)
		errs = append(errs, validatePositiveDuration(base+".read_timeout", redisCfg.ReadTimeout)...)
		errs = append(errs, validatePositiveDuration(base+".write_timeout", redisCfg.WriteTimeout)...)
		errs = append(errs, validatePositiveDuration(base+".ping_timeout", redisCfg.PingTimeout)...)
	}
	return errs
}

func (c Config) validatePostgres() []error {
	var errs []error
	for _, name := range sortedPostgresNames(c.Postgres) {
		pg := c.Postgres[name]
		base := "postgres." + name
		if strings.TrimSpace(name) == "" {
			errs = append(errs, configFieldError("postgres", "must not contain an empty named instance"))
		}
		if strings.TrimSpace(pg.Host) == "" {
			errs = append(errs, configFieldError(base+".host", "is required"))
		}
		errs = append(errs, validatePort(base+".port", pg.Port)...)
		if strings.TrimSpace(pg.Username) == "" {
			errs = append(errs, configFieldError(base+".username", "is required"))
		}
		if strings.TrimSpace(pg.DBName) == "" {
			errs = append(errs, configFieldError(base+".db_name", "is required"))
		}
		driver := strings.ToLower(strings.TrimSpace(pg.Driver))
		if driver == "" {
			errs = append(errs, configFieldError(base+".driver", "is required"))
		} else if _, ok := validPostgresDrivers[driver]; !ok {
			errs = append(errs, configFieldError(base+".driver", "must be pgx"))
		}
		sslMode := strings.ToLower(strings.TrimSpace(pg.SSLMode))
		if sslMode == "" {
			errs = append(errs, configFieldError(base+".sslmode", "is required"))
		} else if _, ok := validPostgresSSLModes[sslMode]; !ok {
			errs = append(errs, configFieldError(base+".sslmode", "must be a valid PostgreSQL sslmode"))
		} else if c.isProductionLike() && sslMode == "disable" {
			errs = append(errs, configFieldError(base+".sslmode", "must not be disable in production-like environments"))
		}
		errs = append(errs, validatePositiveInt(base+".max_open_conns", pg.MaxOpenConns)...)
		errs = append(errs, validateNonNegativeInt(base+".max_idle_conns", pg.MaxIdleConns)...)
		if pg.MaxOpenConns > 0 && pg.MaxIdleConns > pg.MaxOpenConns {
			errs = append(errs, configFieldError(base+".max_idle_conns", "must be <= max_open_conns"))
		}
		errs = append(errs, validatePositiveDuration(base+".conn_max_lifetime", pg.ConnMaxLifetime)...)
		errs = append(errs, validatePositiveDuration(base+".conn_max_idle_time", pg.ConnMaxIdleTime)...)
		errs = append(errs, validatePositiveDuration(base+".ping_timeout", pg.PingTimeout)...)
	}
	return errs
}

func (c Config) isProductionLike() bool {
	_, ok := productionLikeEnvironments[strings.ToLower(strings.TrimSpace(c.App.Environment))]
	return ok
}

func validateHostPort(path string, value string) []error {
	host, portText, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(host) == "" {
		return []error{configFieldError(path, "must be in host:port format")}
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		return []error{configFieldError(path, "port must be valid")}
	}
	return validatePort(path, port)
}

func validatePort(path string, value int) []error {
	if value < minPort || value > maxPort {
		return []error{configFieldError(path, fmt.Sprintf("must be between %d and %d", minPort, maxPort))}
	}
	return nil
}

func validatePositiveDuration(path string, value time.Duration) []error {
	if value <= 0 {
		return []error{configFieldError(path, "must be > 0")}
	}
	return nil
}

func validatePositiveInt(path string, value int) []error {
	if value <= 0 {
		return []error{configFieldError(path, "must be > 0")}
	}
	return nil
}

func validatePositiveInt64(path string, value int64) []error {
	if value <= 0 {
		return []error{configFieldError(path, "must be > 0")}
	}
	return nil
}

func validateNonNegativeInt(path string, value int) []error {
	if value < 0 {
		return []error{configFieldError(path, "must be >= 0")}
	}
	return nil
}

func validateNonNegativeInt64(path string, value int64) []error {
	if value < 0 {
		return []error{configFieldError(path, "must be >= 0")}
	}
	return nil
}

func configFieldError(path string, message string) error {
	return errors.New(path + " " + message)
}

func sortedRedisNames(values map[string]RedisConfig) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedPostgresNames(values map[string]PostgresConfig) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedLocalCacheNames(values LocalCacheConfig) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
