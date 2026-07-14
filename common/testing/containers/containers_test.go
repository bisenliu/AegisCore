package containers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/resources"
)

func TestPostgresContainerConfigUsesResourceContract(t *testing.T) {
	cfg := (PostgresContainer{
		Host:     "127.0.0.1",
		Port:     15432,
		Database: "aegiscore_test",
		Username: "aegiscore",
		Password: "secret",
	}).Config()

	require.Equal(t, resources.DefaultPostgresSSLMode, cfg.SSLMode)
	require.Equal(t, 2, cfg.Pool.MaxOpenConns)
	require.Equal(t, 1, cfg.Pool.MaxIdleConns)
}

func TestRedisContainerConfigUsesResourceContract(t *testing.T) {
	cfg := (RedisContainer{Addr: "127.0.0.1:16379", DB: 2}).Config()

	require.Equal(t, "127.0.0.1:16379", cfg.Addr)
	require.Equal(t, 2, cfg.DB)
	require.Equal(t, resources.DefaultRedisTimeout, cfg.Timeout)
}

func TestStartPostgresIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	pg := StartPostgres(ctx, t, PostgresOptions{})
	db, err := sql.Open("pgx", pg.DSN)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.PingContext(ctx))

	cfg := pg.Config()
	require.NotEmpty(t, cfg.Host)
	require.NotZero(t, cfg.Port)
	require.Equal(t, DefaultPostgresDatabase, cfg.DBName)
	require.Equal(t, "disable", cfg.SSLMode)
	require.Positive(t, cfg.Pool.MaxOpenConns)
}

func TestStartRedisIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	redisContainer := StartRedis(ctx, t, RedisOptions{})
	client := redis.NewClient(redisContainer.Options())
	defer client.Close()
	require.NoError(t, client.Ping(ctx).Err())

	cfg := redisContainer.Config()
	require.NotEmpty(t, cfg.Addr)
	require.Zero(t, cfg.DB)
	require.Positive(t, cfg.Timeout)
}
