package resources

import "time"

const (
	// DefaultRedisTimeout 是 Redis 建连、读写和启动探测共用的默认超时。
	DefaultRedisTimeout = 5 * time.Second

	// DefaultPostgresSSLMode 是未显式配置时使用的 PostgreSQL TLS 模式。
	DefaultPostgresSSLMode = "disable"

	// DefaultPostgresMaxOpenConns 是 PostgreSQL 连接池默认最大打开连接数。
	DefaultPostgresMaxOpenConns = 25
	// DefaultPostgresMaxIdleConns 是 PostgreSQL 连接池默认最大空闲连接数。
	DefaultPostgresMaxIdleConns = 5
	// DefaultPostgresConnMaxLifetime 是 PostgreSQL 连接的默认最长复用时间。
	DefaultPostgresConnMaxLifetime = 30 * time.Minute
	// DefaultPostgresConnMaxIdleTime 是 PostgreSQL 空闲连接的默认保留时间。
	DefaultPostgresConnMaxIdleTime = 10 * time.Minute

	defaultPostgresPingTimeout = 5 * time.Second
)

// DefaultPostgresPingTimeout 返回 PostgreSQL 启动探测的内部默认超时。
// 该值不属于 YAML 配置契约。
func DefaultPostgresPingTimeout() time.Duration {
	return defaultPostgresPingTimeout
}

// ApplyDefaults 为所有 Redis 资源补齐未显式配置的默认值。
func (c RedisConfigs) ApplyDefaults() {
	for name, cfg := range c {
		cfg.ApplyDefaults()
		c[name] = cfg
	}
}

// ApplyDefaults 为 Redis 资源补齐未显式配置的默认值。
func (c *RedisConfig) ApplyDefaults() {
	if c == nil {
		return
	}
	if c.Timeout == 0 {
		c.Timeout = DefaultRedisTimeout
	}
}

// ApplyDefaults 为所有 PostgreSQL 资源补齐未显式配置的默认值。
func (c PostgresConfigs) ApplyDefaults() {
	for name, cfg := range c {
		cfg.ApplyDefaults()
		c[name] = cfg
	}
}

// ApplyDefaults 为 PostgreSQL 资源补齐未显式配置的默认值。
func (c *PostgresConfig) ApplyDefaults() {
	if c == nil {
		return
	}
	if c.SSLMode == "" {
		c.SSLMode = DefaultPostgresSSLMode
	}
	if c.Pool.MaxOpenConns == 0 {
		c.Pool.MaxOpenConns = DefaultPostgresMaxOpenConns
	}
	if c.Pool.MaxIdleConns == 0 {
		c.Pool.MaxIdleConns = DefaultPostgresMaxIdleConns
	}
	if c.Pool.ConnMaxLifetime == 0 {
		c.Pool.ConnMaxLifetime = DefaultPostgresConnMaxLifetime
	}
	if c.Pool.ConnMaxIdleTime == 0 {
		c.Pool.ConnMaxIdleTime = DefaultPostgresConnMaxIdleTime
	}
}
