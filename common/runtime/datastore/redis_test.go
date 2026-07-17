package datastore

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/aegiscore/common/runtime/resources"
)

func TestOpenRedisClientMapsDefaultTimeoutAndCredentials(t *testing.T) {
	client := OpenRedisClient(resources.RedisConfig{
		Addr: "127.0.0.1:6379", Username: "service-account", Password: "secret",
	})
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
	client := OpenRedisClient(resources.RedisConfig{Addr: listener.Addr().String(), Timeout: 10 * time.Millisecond}, WithRedisTracerProvider(provider))
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx, span := provider.Tracer("datastore-test").Start(context.Background(), "parent")
	_ = client.Ping(ctx).Err()
	span.End()
	require.NotEmpty(t, recorder.Ended())
}
