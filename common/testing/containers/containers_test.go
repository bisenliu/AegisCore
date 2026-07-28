package containers

import (
	"context"
	"database/sql"
	"net"
	"strconv"
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
	cfg := (RedisContainer{Addr: "127.0.0.1:16379"}).Config()

	require.Equal(t, resources.RedisModeCluster, cfg.Mode)
	require.Equal(t, []string{"127.0.0.1:16379"}, cfg.Addrs)
	require.Equal(t, resources.DefaultRedisTimeout, cfg.Timeout)
	require.Equal(t, resources.DefaultRedisClusterMaxRedirects, cfg.Cluster.MaxRedirects)
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
	shards, err := client.ClusterShards(ctx).Result()
	require.NoError(t, err)
	require.NotEmpty(t, shards)
	require.NotEmpty(t, shards[0].Nodes)
	advertisedNode := shards[0].Nodes[0]
	require.Equal(t, redisContainer.Addr, net.JoinHostPort(advertisedNode.IP, strconv.FormatInt(advertisedNode.Port, 10)))

	cfg := redisContainer.Config()
	require.NotEmpty(t, cfg.Addrs)
	require.Positive(t, cfg.Timeout)
	clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        cfg.Addrs,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
		MaxRedirects: cfg.Cluster.MaxRedirects,
	})
	defer clusterClient.Close()
	require.NoError(t, clusterClient.Ping(ctx).Err())
}
