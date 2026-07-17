package resources

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRedisConfigsApplyDefaultsAndValidateMultipleResources(t *testing.T) {
	configs := RedisConfigs{
		"cache_redis": {
			Addr: "127.0.0.1:6379",
			DB:   0,
		},
		"queue_redis": {
			Addr:     "redis.internal:6380",
			Username: "",
			Password: "",
			DB:       1,
			Timeout:  9 * time.Second,
		},
	}

	configs.ApplyDefaults()

	require.Equal(t, DefaultRedisTimeout, configs["cache_redis"].Timeout)
	require.Equal(t, 9*time.Second, configs["queue_redis"].Timeout)
	require.NoError(t, configs.Validate("resources.redis"))
}

func TestRedisConfigsValidateRejectsInvalidResources(t *testing.T) {
	configs := RedisConfigs{
		"": {
			Addr:    "missing-port",
			DB:      -1,
			Timeout: -time.Second,
		},
		"cache_redis": {
			Addr:    ":6379",
			Timeout: time.Second,
		},
		"queue_redis": {
			Addr:    "redis.internal:70000",
			Timeout: 0,
		},
	}

	err := configs.Validate("resources.redis")
	require.Error(t, err)
	require.ErrorContains(t, err, "resources.redis must not contain an empty named resource")
	require.ErrorContains(t, err, "resources.redis.addr must be in host:port format")
	require.ErrorContains(t, err, "resources.redis.db must be >= 0")
	require.ErrorContains(t, err, "resources.redis.timeout must be > 0")
	require.ErrorContains(t, err, "resources.redis.cache_redis.addr must be in host:port format")
	require.ErrorContains(t, err, "resources.redis.queue_redis.addr port must be valid")
	require.ErrorContains(t, err, "resources.redis.queue_redis.timeout must be > 0")
}

func TestPostgresConfigsApplyDefaultsAndValidateMultipleResources(t *testing.T) {
	configs := PostgresConfigs{
		"primary_db": {
			Host:     "127.0.0.1",
			Port:     5432,
			Username: "user",
			Password: "",
			DBName:   "aegiscore_user",
		},
		"audit_db": {
			Host:     "postgres.internal",
			Port:     5433,
			Username: "audit",
			Password: "secret",
			DBName:   "aegiscore_audit",
			SSLMode:  "verify-full",
			Pool: PostgresPoolConfig{
				MaxOpenConns:    12,
				MaxIdleConns:    3,
				ConnMaxLifetime: time.Hour,
				ConnMaxIdleTime: 15 * time.Minute,
			},
		},
	}

	configs.ApplyDefaults()

	userDB := configs["primary_db"]
	require.Equal(t, DefaultPostgresSSLMode, userDB.SSLMode)
	require.Equal(t, DefaultPostgresMaxOpenConns, userDB.Pool.MaxOpenConns)
	require.Equal(t, DefaultPostgresMaxIdleConns, userDB.Pool.MaxIdleConns)
	require.Equal(t, DefaultPostgresConnMaxLifetime, userDB.Pool.ConnMaxLifetime)
	require.Equal(t, DefaultPostgresConnMaxIdleTime, userDB.Pool.ConnMaxIdleTime)
	require.Equal(t, 12, configs["audit_db"].Pool.MaxOpenConns)
	require.NoError(t, configs.Validate("resources.postgres"))
}

func TestPostgresConfigsValidateRejectsInvalidResources(t *testing.T) {
	configs := PostgresConfigs{
		"": {
			Port:    0,
			SSLMode: "invalid",
			Pool: PostgresPoolConfig{
				MaxOpenConns:    -1,
				MaxIdleConns:    -1,
				ConnMaxLifetime: -time.Second,
				ConnMaxIdleTime: -time.Second,
			},
		},
		"primary_db": {
			Host:     "127.0.0.1",
			Port:     70000,
			Username: " ",
			DBName:   "aegiscore_user",
			SSLMode:  "require",
			Pool: PostgresPoolConfig{
				MaxOpenConns:    2,
				MaxIdleConns:    3,
				ConnMaxLifetime: time.Minute,
				ConnMaxIdleTime: time.Minute,
			},
		},
	}

	err := configs.Validate("resources.postgres")
	require.Error(t, err)
	require.ErrorContains(t, err, "resources.postgres must not contain an empty named resource")
	require.ErrorContains(t, err, "resources.postgres.host is required")
	require.ErrorContains(t, err, "resources.postgres.port must be between 1 and 65535")
	require.ErrorContains(t, err, "resources.postgres.db_name is required")
	require.ErrorContains(t, err, "resources.postgres.sslmode must be a valid PostgreSQL sslmode")
	require.ErrorContains(t, err, "resources.postgres.pool.max_open_conns must be > 0")
	require.ErrorContains(t, err, "resources.postgres.pool.max_idle_conns must be >= 0")
	require.ErrorContains(t, err, "resources.postgres.pool.conn_max_lifetime must be > 0")
	require.ErrorContains(t, err, "resources.postgres.pool.conn_max_idle_time must be > 0")
	require.ErrorContains(t, err, "resources.postgres.primary_db.port must be between 1 and 65535")
	require.ErrorContains(t, err, "resources.postgres.primary_db.username is required")
	require.ErrorContains(t, err, "resources.postgres.primary_db.pool.max_idle_conns must be <= max_open_conns")
}

func TestPostgresPingTimeoutIsInternalHelperOnly(t *testing.T) {
	require.Equal(t, 5*time.Second, DefaultPostgresPingTimeout())

	typeOfConfig := reflect.TypeOf(PostgresConfig{})
	for i := 0; i < typeOfConfig.NumField(); i++ {
		field := typeOfConfig.Field(i)
		require.NotEqual(t, "ping_timeout", field.Tag.Get("mapstructure"))
	}
}
