package config

import (
	"net"
	"net/url"
	"strconv"
	"time"
)

// Config is the root configuration object for AegisCore services.
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	HTTP     HTTPConfig     `mapstructure:"http"`
	Log      LogConfig      `mapstructure:"log"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Database DatabaseConfig `mapstructure:"database"`
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
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Postgres PostgresConfig `mapstructure:"postgres"`
}

type PostgresConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	UserDBName      string        `mapstructure:"user_db_name"`
	PayDBName       string        `mapstructure:"pay_db_name"`
	CommonDBName    string        `mapstructure:"common_db_name"`
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

func (c Config) Postgres(name string) (PostgresDatabaseConfig, bool) {
	return c.Database.Postgres.Database(name)
}

func (p PostgresConfig) Database(name string) (PostgresDatabaseConfig, bool) {
	dbName := p.databaseName(name)
	if dbName == "" {
		return PostgresDatabaseConfig{}, false
	}
	return PostgresDatabaseConfig{
		Driver:          p.Driver,
		DSN:             p.dsn(dbName),
		MaxOpenConns:    p.MaxOpenConns,
		MaxIdleConns:    p.MaxIdleConns,
		ConnMaxLifetime: p.ConnMaxLifetime,
		ConnMaxIdleTime: p.ConnMaxIdleTime,
		PingTimeout:     p.PingTimeout,
	}, true
}

func (p PostgresConfig) databaseName(name string) string {
	switch name {
	case "user_db":
		return p.UserDBName
	case "pay_db":
		return p.PayDBName
	case "common_db":
		return p.CommonDBName
	default:
		return ""
	}
}

func (p PostgresConfig) dsn(dbName string) string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(p.Username, p.Password),
		Host:   net.JoinHostPort(p.Host, strconv.Itoa(p.Port)),
		Path:   dbName,
	}
	if p.SSLMode != "" {
		q := u.Query()
		q.Set("sslmode", p.SSLMode)
		u.RawQuery = q.Encode()
	}
	return u.String()
}
