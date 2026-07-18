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
		Addr: "127.0.0.1:6379", Username: "service-account", Password: "secret",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	opts := client.Options()
	require.Equal(t, "service-account", opts.Username)
	require.Equal(t, "secret", opts.Password)
	require.Equal(t, resources.DefaultRedisTimeout, opts.DialTimeout)
	require.Equal(t, resources.DefaultRedisTimeout, opts.ReadTimeout)
	require.Equal(t, resources.DefaultRedisTimeout, opts.WriteTimeout)
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
	client, err := OpenRedisClient(resources.RedisConfig{Addr: listener.Addr().String(), Timeout: 10 * time.Millisecond}, WithRedisTracerProvider(provider))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx, span := provider.Tracer("datastore-test").Start(context.Background(), "parent")
	_ = client.Ping(ctx).Err()
	span.End()
	require.NotEmpty(t, recorder.Ended())
}

func TestOpenRedisClientReturnsErrorWhenInstrumentationFails(t *testing.T) {
	instrumentErr := errors.New("instrumentation failed")
	var captured *redis.Client

	var client *redis.Client
	require.NotPanics(t, func() {
		var err error
		client, err = OpenRedisClient(
			resources.RedisConfig{Addr: "127.0.0.1:6379"},
			withRedisInstrumenterForTest(func(redisClient *redis.Client, _ trace.TracerProvider) error {
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
		resources.RedisConfig{Addr: "127.0.0.1:6379"},
		withRedisInstrumenterForTest(func(redisClient *redis.Client, _ trace.TracerProvider) error {
			require.NoError(t, redisClient.Close())
			return instrumentErr
		}),
	)
	require.Nil(t, client)
	require.Error(t, err)
	require.ErrorIs(t, err, instrumentErr)
	require.ErrorIs(t, err, redis.ErrClosed)
}

func withRedisInstrumenterForTest(instrument redisInstrumenter) RedisClientOption {
	return func(opts *redisClientOptions) {
		if instrument != nil {
			opts.instrument = instrument
		}
	}
}
