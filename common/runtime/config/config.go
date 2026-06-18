package config

import (
	"net"
	"net/url"
	"strconv"
	"time"
)

// Config 是 AegisCore 服务的根配置对象。
type Config struct {
	System        SystemConfig              `mapstructure:"system"`
	App           AppConfig                 `mapstructure:"app"`
	HTTP          HTTPConfig                `mapstructure:"http"`
	Auth          AuthConfig                `mapstructure:"auth"`
	Ent           EntConfig                 `mapstructure:"ent"`
	Log           LogConfig                 `mapstructure:"log"`
	Observability ObservabilityConfig       `mapstructure:"observability"`
	Redis         map[string]RedisConfig    `mapstructure:"redis"`
	Postgres      map[string]PostgresConfig `mapstructure:"postgres"`
}

// SystemConfig 包含进程级运行时设置。
type SystemConfig struct {
	Timezone string `mapstructure:"timezone"`
}

// AppConfig 标识运行中的服务和部署环境。
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

// HTTPConfig 包含服务地址、超时、关闭、代理和诊断端点设置。
type HTTPConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	TrustedProxies  []string      `mapstructure:"trusted_proxies"`
	Pprof           PprofConfig   `mapstructure:"pprof"`
}

// PprofConfig 控制服务是否挂载 Go runtime pprof 诊断端点。
type PprofConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BasePath string `mapstructure:"base_path"`
}

// AuthConfig 包含认证 token 与会话校验设置。
type AuthConfig struct {
	JWT                      JWTConfig     `mapstructure:"jwt"`
	TokenVersionCacheTTL     time.Duration `mapstructure:"token_version_cache_ttl"`
	RefreshTokenRotation     bool          `mapstructure:"refresh_token_rotation"`
	MaxActiveSessionsPerUser int           `mapstructure:"max_active_sessions_per_user"`
}

// EntConfig 控制 Ent 运行时行为。
type EntConfig struct {
	SQLDebug bool `mapstructure:"sql_debug"`
}

// JWTConfig 包含 JWT 签发和校验设置。
type JWTConfig struct {
	Secret          string        `mapstructure:"secret"`
	Issuer          string        `mapstructure:"issuer"`
	Audience        string        `mapstructure:"audience"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
}

// LogConfig 控制 zap logger 格式、输出目标和文件轮转。
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Directory  string `mapstructure:"directory"`
	Filename   string `mapstructure:"filename"`
	Console    bool   `mapstructure:"console"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
}

// ObservabilityConfig 包含可观测性运行时配置入口。
type ObservabilityConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing"`
}

// MetricsConfig 包含 metrics 采集和暴露的配置契约。
type MetricsConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	Path           string `mapstructure:"path"`
	IncludeRuntime bool   `mapstructure:"include_runtime"`
}

// TracingConfig 包含 tracing 采样和 exporter 的配置契约。
type TracingConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	SampleRatio  float64 `mapstructure:"sample_ratio"`
	Exporter     string  `mapstructure:"exporter"`
	OTLPEndpoint string  `mapstructure:"otlp_endpoint"`
	Insecure     bool    `mapstructure:"insecure"`
}

// RedisConfig 包含一个具名 Redis 客户端配置。
type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PingTimeout  time.Duration `mapstructure:"ping_timeout"`
}

// PostgresConfig 包含一个具名 PostgreSQL 数据源在生成 DSN 前的配置。
type PostgresConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"db_name"`
	Driver          string        `mapstructure:"driver"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	PingTimeout     time.Duration `mapstructure:"ping_timeout"`
}

// PostgresDBConfig 包含从 PostgresConfig 派生出的 SQL driver 设置。
type PostgresDBConfig struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingTimeout     time.Duration
}

// RedisConfig 返回具名 Redis 配置及其是否存在。
func (c Config) RedisConfig(name string) (RedisConfig, bool) {
	redisCfg, ok := c.Redis[name]
	return redisCfg, ok
}

// PostgresDatabaseConfig 返回带有已生成 DSN 的具名 PostgreSQL 数据库配置。
func (c Config) PostgresDatabaseConfig(name string) (PostgresDBConfig, bool) {
	pg, ok := c.Postgres[name]
	if !ok {
		return PostgresDBConfig{}, false
	}
	return PostgresDBConfig{
		Driver:          pg.Driver,
		DSN:             pg.dsn(),
		MaxOpenConns:    pg.MaxOpenConns,
		MaxIdleConns:    pg.MaxIdleConns,
		ConnMaxLifetime: pg.ConnMaxLifetime,
		ConnMaxIdleTime: pg.ConnMaxIdleTime,
		PingTimeout:     pg.PingTimeout,
	}, true
}

func (p PostgresConfig) dsn() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(p.Username, p.Password),
		Host:   net.JoinHostPort(p.Host, strconv.Itoa(p.Port)),
		Path:   p.DBName,
	}
	if p.SSLMode != "" {
		// 未配置 sslmode 时保持为空，让 pgx driver 使用自身默认行为。
		q := u.Query()
		q.Set("sslmode", p.SSLMode)
		u.RawQuery = q.Encode()
	}
	return u.String()
}
