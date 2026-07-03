package containers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestStartPostgresIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	pg := StartPostgres(ctx, t, PostgresOptions{})
	db, err := sql.Open("pgx", pg.DSN)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.PingContext(ctx))

	cfg := pg.Config()
	require.Equal(t, "pgx", cfg.Driver)
	require.NotEmpty(t, cfg.Host)
	require.NotZero(t, cfg.Port)
	require.Equal(t, DefaultPostgresDatabase, cfg.DBName)
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
	require.Positive(t, cfg.PingTimeout)
}
