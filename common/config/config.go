package config

import "time"

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
	Postgres map[string]PostgresConfig `mapstructure:"postgres"`
}

type PostgresConfig struct {
	Driver          string        `mapstructure:"driver"`
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	PingTimeout     time.Duration `mapstructure:"ping_timeout"`
}

func (c Config) Postgres(name string) (PostgresConfig, bool) {
	db, ok := c.Database.Postgres[name]
	return db, ok
}
