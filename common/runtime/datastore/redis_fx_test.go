package datastore

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/resources"
)

const testCacheRedis = "cache_redis"

func TestNewRedisClientRegistersLifecycle(t *testing.T) {
	server := miniredis.RunT(t)
	lc := fxtest.NewLifecycle(t)
	client := NewRedisClient(lc, zap.NewNop(), testCacheRedis, resources.RedisConfig{Addr: server.Addr()})
	lc.RequireStart()
	lc.RequireStop()
	require.Equal(t, resources.DefaultRedisTimeout, client.Options().DialTimeout)
	require.ErrorIs(t, client.Ping(context.Background()).Err(), redis.ErrClosed)
}

func TestNewRedisClientClosesClientWhenStartPingFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	lc := fxtest.NewLifecycle(t)
	client := NewRedisClient(lc, zap.NewNop(), testCacheRedis, resources.RedisConfig{Addr: addr, Timeout: 20 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Start(ctx)
	require.ErrorContains(t, err, "ping redis cache_redis")
	require.ErrorIs(t, client.Ping(ctx).Err(), redis.ErrClosed)
}
