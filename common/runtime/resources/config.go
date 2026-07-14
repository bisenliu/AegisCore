package resources

import "time"

// RedisConfigs 是按资源名索引的 Redis 配置集合。
type RedisConfigs map[string]RedisConfig

// RedisConfig 描述单个 Redis 资源的连接参数。
type RedisConfig struct {
	Addr     string        `mapstructure:"addr"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
	DB       int           `mapstructure:"db"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// PostgresConfigs 是按资源名索引的 PostgreSQL 配置集合。
type PostgresConfigs map[string]PostgresConfig

// PostgresConfig 描述单个 PostgreSQL 资源的连接和连接池参数。
type PostgresConfig struct {
	Host     string             `mapstructure:"host"`
	Port     int                `mapstructure:"port"`
	Username string             `mapstructure:"username"`
	Password string             `mapstructure:"password"`
	DBName   string             `mapstructure:"db_name"`
	SSLMode  string             `mapstructure:"sslmode"`
	Pool     PostgresPoolConfig `mapstructure:"pool"`
}

// PostgresPoolConfig 描述 database/sql 连接池参数。
type PostgresPoolConfig struct {
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}
