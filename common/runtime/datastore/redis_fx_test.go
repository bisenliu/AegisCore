package datastore

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/resources"
)

const testCacheRedis = "cache_redis"

func TestNewRedisClientRegistersLifecycle(t *testing.T) {
	server := miniredis.RunT(t)
	lc := fxtest.NewLifecycle(t)
	client, err := NewRedisClient(lc, zap.NewNop(), testCacheRedis, resources.RedisConfig{Addr: server.Addr()})
	require.NoError(t, err)
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
	client, err := NewRedisClient(lc, zap.NewNop(), testCacheRedis, resources.RedisConfig{Addr: addr, Timeout: 20 * time.Millisecond})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Start(ctx)
	require.ErrorContains(t, err, "ping redis cache_redis")
	require.ErrorIs(t, client.Ping(ctx).Err(), redis.ErrClosed)
}

func TestNewRedisClientClosesClientWhenLaterStartHookFails(t *testing.T) {
	server := miniredis.RunT(t)
	var client *redis.Client
	startErr := errors.New("later start failed")
	app := fxtest.New(t,
		fx.NopLogger,
		fx.Supply(zap.NewNop()),
		fx.Provide(func(lifecycle fx.Lifecycle, log *zap.Logger) (*redis.Client, error) {
			return NewRedisClient(lifecycle, log, testCacheRedis, resources.RedisConfig{Addr: server.Addr()})
		}),
		fx.Populate(&client),
		fx.Invoke(func(lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{OnStart: func(context.Context) error { return startErr }})
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := app.Start(ctx)
	require.ErrorIs(t, err, startErr)
	require.ErrorIs(t, client.Ping(ctx).Err(), redis.ErrClosed)
}

func TestNewRedisClientReturnsConstructorError(t *testing.T) {
	instrumentErr := errors.New("instrumentation failed")
	lc := fxtest.NewLifecycle(t)

	client, err := NewRedisClient(
		lc,
		zap.NewNop(),
		testCacheRedis,
		resources.RedisConfig{Addr: "127.0.0.1:6379"},
		withRedisInstrumenterForTest(func(*redis.Client, trace.TracerProvider) error {
			return instrumentErr
		}),
	)
	require.Nil(t, client)
	require.ErrorContains(t, err, "open redis cache_redis")
	require.ErrorContains(t, err, "instrument redis tracing")
	require.ErrorIs(t, err, instrumentErr)
	lc.RequireStart()
	lc.RequireStop()
}
