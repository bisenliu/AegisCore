package resources

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRedisConfigsApplyDefaultsAndValidateMultipleResources(t *testing.T) {
	configs := RedisConfigs{
		"cache_redis": {Mode: RedisModeStandalone, Addr: "127.0.0.1:6379"},
		"queue_redis": {
			Mode:     RedisModeCluster,
			Addrs:    []string{"redis.internal:6380"},
			Username: "",
			Password: "",
			Timeout:  9 * time.Second,
			Cluster:  RedisClusterConfig{MaxRedirects: 12},
		},
	}

	configs.ApplyDefaults()

	require.Equal(t, DefaultRedisTimeout, configs["cache_redis"].Timeout)
	require.Equal(t, RedisModeStandalone, configs["cache_redis"].Mode)
	require.Zero(t, configs["cache_redis"].Cluster.MaxRedirects)
	require.Equal(t, 9*time.Second, configs["queue_redis"].Timeout)
	require.Equal(t, 12, configs["queue_redis"].Cluster.MaxRedirects)
	require.NoError(t, configs.Validate("resources.redis"))
}

func TestRedisConfigsValidateRejectsInvalidResources(t *testing.T) {
	configs := RedisConfigs{
		"":            {Mode: "unknown", Addrs: []string{"missing-port"}, Timeout: -time.Second, Cluster: RedisClusterConfig{MaxRedirects: -1}},
		"cache_redis": {Mode: RedisModeCluster, Addrs: []string{":6379"}, Timeout: time.Second},
		"queue_redis": {Mode: RedisModeCluster, Addrs: []string{"redis.internal:70000"}, Timeout: 0},
		"empty_redis": {Mode: RedisModeCluster, Timeout: time.Second},
		"bad_standalone": {Mode: RedisModeStandalone, Addr: ":6379", Addrs: []string{"127.0.0.1:6379"}, Timeout: time.Second,
			Cluster: RedisClusterConfig{MaxRedirects: 1}},
	}

	err := configs.Validate("resources.redis")
	require.Error(t, err)
	require.ErrorContains(t, err, "resources.redis must not contain an empty named resource")
	require.ErrorContains(t, err, "resources.redis.mode must be standalone or cluster")
	require.ErrorContains(t, err, "resources.redis.timeout must be > 0")
	require.ErrorContains(t, err, "resources.redis.bad_standalone.addr must be in host:port format")
	require.ErrorContains(t, err, "resources.redis.bad_standalone.addrs must be empty when mode is standalone")
	require.ErrorContains(t, err, "resources.redis.bad_standalone.cluster.max_redirects must be 0 when mode is standalone")
	require.ErrorContains(t, err, "resources.redis.cache_redis.addrs[0] must be in host:port format")
	require.ErrorContains(t, err, "resources.redis.queue_redis.addrs[0] port must be valid")
	require.ErrorContains(t, err, "resources.redis.queue_redis.timeout must be > 0")
	require.ErrorContains(t, err, "resources.redis.empty_redis.addrs must contain at least one address")
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
