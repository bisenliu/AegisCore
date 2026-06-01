package config

import (
	"net"
	"net/url"
	"strconv"
	"time"
)

// Config is the root configuration object for AegisCore services.
type Config struct {
	System          SystemConfig              `mapstructure:"system"`
	App             AppConfig                 `mapstructure:"app"`
	HTTP            HTTPConfig                `mapstructure:"http"`
	Log             LogConfig                 `mapstructure:"log"`
	Redis           map[string]RedisConfig    `mapstructure:"redis"`
	PostgresConfigs map[string]PostgresConfig `mapstructure:"postgres"`
}

type SystemConfig struct {
	Timezone string `mapstructure:"timezone"`
}

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

type HTTPConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	TrustedProxies  []string      `mapstructure:"trusted_proxies"`
}

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

type PostgresDatabaseConfig struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingTimeout     time.Duration
}

func (c Config) RedisConfig(name string) (RedisConfig, bool) {
	redisCfg, ok := c.Redis[name]
	return redisCfg, ok
}

func (c Config) Postgres(name string) (PostgresDatabaseConfig, bool) {
	pg, ok := c.PostgresConfigs[name]
	if !ok {
		return PostgresDatabaseConfig{}, false
	}
	return PostgresDatabaseConfig{
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
		q := u.Query()
		q.Set("sslmode", p.SSLMode)
		u.RawQuery = q.Encode()
	}
	return u.String()
}
