package config

import (
	"net"
	"net/url"
	"strconv"
	"time"
)

// Config is the root configuration object for AegisCore services.
type Config struct {
	App      AppConfig      `mapstructure:"app" validate:"required"`
	HTTP     HTTPConfig     `mapstructure:"http" validate:"required"`
	Log      LogConfig      `mapstructure:"log" validate:"required"`
	Redis    RedisConfig    `mapstructure:"redis" validate:"required"`
	Database DatabaseConfig `mapstructure:"database" validate:"required"`
}

type AppConfig struct {
	Name        string `mapstructure:"name" validate:"required"`
	Environment string `mapstructure:"environment" validate:"required"`
}

type HTTPConfig struct {
	Host            string        `mapstructure:"host" validate:"required"`
	Port            int           `mapstructure:"port" validate:"min=1,max=65535"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout" validate:"gt=0"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout" validate:"gt=0"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout" validate:"gt=0"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" validate:"gt=0"`
	TrustedProxies  []string      `mapstructure:"trusted_proxies"`
}

type LogConfig struct {
	Level  string `mapstructure:"level" validate:"required"`
	Format string `mapstructure:"format" validate:"required"`
}

type RedisConfig struct {
	Addr         string        `mapstructure:"addr" validate:"required"`
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db" validate:"min=0"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout" validate:"gt=0"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" validate:"gt=0"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" validate:"gt=0"`
}

type DatabaseConfig struct {
	Postgres PostgresConfig `mapstructure:"postgres" validate:"required"`
}

type PostgresConfig struct {
	Host            string        `mapstructure:"host" validate:"required"`
	Port            int           `mapstructure:"port" validate:"min=1,max=65535"`
	Username        string        `mapstructure:"username" validate:"required"`
	Password        string        `mapstructure:"password"`
	UserDBName      string        `mapstructure:"user_db_name" validate:"required"`
	PayDBName       string        `mapstructure:"pay_db_name" validate:"required"`
	CommonDBName    string        `mapstructure:"common_db_name" validate:"required"`
	Driver          string        `mapstructure:"driver" validate:"required"`
	SSLMode         string        `mapstructure:"sslmode" validate:"required"`
	MaxOpenConns    int           `mapstructure:"max_open_conns" validate:"gt=0"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns" validate:"gt=0"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime" validate:"gt=0"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time" validate:"gt=0"`
	PingTimeout     time.Duration `mapstructure:"ping_timeout" validate:"gt=0"`
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
