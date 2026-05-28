package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

const envPrefix = "AEGISCORE"

func Load(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigType("yaml")
	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath("./configs")
		v.AddConfigPath("../configs")
	}

	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := applyDefaults(&cfg); err != nil {
		return nil, err
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "aegiscore")
	v.SetDefault("app.environment", "local")
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.read_timeout", "10s")
	v.SetDefault("http.write_timeout", "10s")
	v.SetDefault("http.idle_timeout", "60s")
	v.SetDefault("http.shutdown_timeout", "10s")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("database.postgres.driver", "pgx")
	v.SetDefault("database.postgres.port", 5432)
	v.SetDefault("database.postgres.sslmode", "disable")
	v.SetDefault("database.postgres.max_open_conns", 25)
	v.SetDefault("database.postgres.max_idle_conns", 5)
	v.SetDefault("database.postgres.conn_max_lifetime", "30m")
	v.SetDefault("database.postgres.conn_max_idle_time", "10m")
	v.SetDefault("database.postgres.ping_timeout", "5s")
}

func applyDefaults(cfg *Config) error {
	db := cfg.Database.Postgres
	if db.Driver == "" {
		db.Driver = "pgx"
	}
	if db.Port == 0 {
		db.Port = 5432
	}
	if db.SSLMode == "" {
		db.SSLMode = "disable"
	}
	if db.MaxOpenConns == 0 {
		db.MaxOpenConns = 25
	}
	if db.MaxIdleConns == 0 {
		db.MaxIdleConns = 5
	}
	if db.ConnMaxLifetime == 0 {
		db.ConnMaxLifetime = 30 * time.Minute
	}
	if db.ConnMaxIdleTime == 0 {
		db.ConnMaxIdleTime = 10 * time.Minute
	}
	if db.PingTimeout == 0 {
		db.PingTimeout = 5 * time.Second
	}
	if db.Host == "" {
		return fmt.Errorf("database.postgres.host is required")
	}
	if db.Port < 1 || db.Port > 65535 {
		return fmt.Errorf("database.postgres.port must be between 1 and 65535")
	}
	if db.Username == "" {
		return fmt.Errorf("database.postgres.username is required")
	}
	if db.UserDBName == "" {
		return fmt.Errorf("database.postgres.user_db_name is required")
	}
	if db.CommonDBName == "" {
		return fmt.Errorf("database.postgres.common_db_name is required")
	}
	cfg.Database.Postgres = db
	return nil
}
