package datastore

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/aegiscore/common/runtime/resources"
)

func TestOpenRedisClientMapsDefaultTimeoutAndCredentials(t *testing.T) {
	client, err := OpenRedisClient(resources.RedisConfig{
		Mode: resources.RedisModeCluster, Addrs: []string{"127.0.0.1:6379"}, Username: "service-account", Password: "secret",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	clusterClient, ok := client.(*redis.ClusterClient)
	require.True(t, ok)
	opts := clusterClient.Options()
	require.Equal(t, "service-account", opts.Username)
	require.Equal(t, "secret", opts.Password)
	require.Equal(t, []string{"127.0.0.1:6379"}, opts.Addrs)
	require.Equal(t, resources.DefaultRedisTimeout, opts.DialTimeout)
	require.Equal(t, resources.DefaultRedisTimeout, opts.ReadTimeout)
	require.Equal(t, resources.DefaultRedisTimeout, opts.WriteTimeout)
	require.Equal(t, resources.DefaultRedisClusterMaxRedirects, opts.MaxRedirects)
}

func TestOpenRedisClientSupportsStandaloneMode(t *testing.T) {
	client, err := OpenRedisClient(resources.RedisConfig{
		Mode: resources.RedisModeStandalone, Addr: "127.0.0.1:6379", Username: "service-account", Password: "secret",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	standaloneClient, ok := client.(*redis.Client)
	require.True(t, ok)
	opts := standaloneClient.Options()
	require.Equal(t, "127.0.0.1:6379", opts.Addr)
	require.Equal(t, "service-account", opts.Username)
	require.Equal(t, "secret", opts.Password)
	require.Zero(t, opts.DB)
	require.Equal(t, resources.DefaultRedisTimeout, opts.DialTimeout)
}

func TestOpenRedisClientUsesExplicitTracerProvider(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(noop.NewTracerProvider())
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	client, err := OpenRedisClient(resources.RedisConfig{Mode: resources.RedisModeCluster, Addrs: []string{listener.Addr().String()}, Timeout: 10 * time.Millisecond}, WithRedisTracerProvider(provider))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx, span := provider.Tracer("datastore-test").Start(context.Background(), "parent")
	_ = client.Ping(ctx).Err()
	span.End()
	require.NotEmpty(t, recorder.Ended())
}

func TestOpenRedisClientReturnsErrorWhenInstrumentationFails(t *testing.T) {
	instrumentErr := errors.New("instrumentation failed")
	var captured redis.UniversalClient

	var client redis.UniversalClient
	require.NotPanics(t, func() {
		var err error
		client, err = OpenRedisClient(
			resources.RedisConfig{Mode: resources.RedisModeCluster, Addrs: []string{"127.0.0.1:6379"}},
			withRedisInstrumenterForTest(func(redisClient redis.UniversalClient, _ trace.TracerProvider) error {
				captured = redisClient
				return instrumentErr
			}),
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "instrument redis tracing")
		require.ErrorIs(t, err, instrumentErr)
	})
	require.Nil(t, client)
	require.NotNil(t, captured)
	require.ErrorIs(t, captured.Ping(context.Background()).Err(), redis.ErrClosed)
}

func TestOpenRedisClientPreservesCloseFailureAfterInstrumentationFails(t *testing.T) {
	instrumentErr := errors.New("instrumentation failed")

	client, err := OpenRedisClient(
		resources.RedisConfig{Mode: resources.RedisModeCluster, Addrs: []string{"127.0.0.1:6379"}},
		withRedisInstrumenterForTest(func(redisClient redis.UniversalClient, _ trace.TracerProvider) error {
			require.NoError(t, redisClient.Close())
			return instrumentErr
		}),
	)
	require.Nil(t, client)
	require.Error(t, err)
	require.ErrorIs(t, err, instrumentErr)
}

func TestOmitRedisCommandTrace(t *testing.T) {
	tests := []struct {
		name string
		cmd  redis.Cmder
		want bool
	}{
		{name: "auth is filtered", cmd: redis.NewCmd(context.Background(), "auth", "secret"), want: true},
		{name: "hello auth is filtered", cmd: redis.NewCmd(context.Background(), "hello", "3", "auth", "user", "secret"), want: true},
		{name: "ping is filtered", cmd: redis.NewCmd(context.Background(), "ping"), want: true},
		{name: "get is traced", cmd: redis.NewCmd(context.Background(), "get", "key"), want: false},
		{name: "nil is filtered", cmd: nil, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, omitRedisCommandTrace(tt.cmd))
		})
	}
}

func withRedisInstrumenterForTest(instrument redisInstrumenter) RedisClientOption {
	return func(opts *redisClientOptions) {
		if instrument != nil {
			opts.instrument = instrument
		}
	}
}
