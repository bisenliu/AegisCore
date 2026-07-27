package datastore

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

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
	server := newTestRedisClusterServer(t)
	lc := fxtest.NewLifecycle(t)
	client, err := NewRedisClient(lc, zap.NewNop(), testCacheRedis, resources.RedisConfig{Mode: resources.RedisModeCluster, Addrs: []string{server.addr}})
	require.NoError(t, err)
	lc.RequireStart()
	lc.RequireStop()
	clusterClient, ok := client.(*redis.ClusterClient)
	require.True(t, ok)
	require.Equal(t, resources.DefaultRedisTimeout, clusterClient.Options().DialTimeout)
	require.ErrorIs(t, client.Ping(context.Background()).Err(), redis.ErrClosed)
}

func TestNewRedisClientClosesClientWhenStartPingFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	lc := fxtest.NewLifecycle(t)
	client, err := NewRedisClient(lc, zap.NewNop(), testCacheRedis, resources.RedisConfig{Mode: resources.RedisModeCluster, Addrs: []string{addr}, Timeout: 20 * time.Millisecond})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Start(ctx)
	require.ErrorContains(t, err, "ping redis cache_redis")
	require.ErrorIs(t, client.Ping(ctx).Err(), redis.ErrClosed)
}

func TestNewRedisClientClosesClientWhenLaterStartHookFails(t *testing.T) {
	server := newTestRedisClusterServer(t)
	var client redis.UniversalClient
	startErr := errors.New("later start failed")
	app := fxtest.New(t,
		fx.NopLogger,
		fx.Supply(zap.NewNop()),
		fx.Provide(func(lifecycle fx.Lifecycle, log *zap.Logger) (redis.UniversalClient, error) {
			return NewRedisClient(lifecycle, log, testCacheRedis, resources.RedisConfig{Mode: resources.RedisModeCluster, Addrs: []string{server.addr}})
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
	require.Error(t, client.Ping(ctx).Err())
}

type testRedisClusterServer struct {
	addr string
}

func newTestRedisClusterServer(t *testing.T) *testRedisClusterServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &testRedisClusterServer{addr: listener.Addr().String()}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go server.handle(conn)
		}
	}()
	return server
}

func (s *testRedisClusterServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		if err := s.writeResponses(conn, strings.ToUpper(string(buf[:n]))); err != nil {
			return
		}
	}
}

func (s *testRedisClusterServer) writeResponses(w io.Writer, command string) error {
	responded := false
	for {
		idx, name := nextTestRedisCommand(command)
		if name == "" {
			if responded {
				return nil
			}
			_, err := w.Write([]byte("+OK\r\n"))
			return err
		}
		responded = true
		command = command[idx+len(name):]
		switch name {
		case "COMMAND":
			if _, err := w.Write([]byte("*0\r\n")); err != nil {
				return err
			}
		case "PING":
			if _, err := w.Write([]byte("+PONG\r\n")); err != nil {
				return err
			}
		case "CLUSTER":
			_, portText, err := net.SplitHostPort(s.addr)
			if err != nil {
				return err
			}
			response := "*1\r\n*3\r\n:0\r\n:16383\r\n*3\r\n$9\r\n127.0.0.1\r\n:" + portText + "\r\n$40\r\n0000000000000000000000000000000000000000\r\n"
			if _, err := w.Write([]byte(response)); err != nil {
				return err
			}
		case "HELLO":
			if _, err := w.Write([]byte("-ERR unknown command 'HELLO'\r\n")); err != nil {
				return err
			}
		default:
			if _, err := w.Write([]byte("+OK\r\n")); err != nil {
				return err
			}
		}
	}
}

func nextTestRedisCommand(command string) (int, string) {
	candidates := []string{"COMMAND", "PING", "HELLO", "CLIENT", "CLUSTER"}
	bestIndex := -1
	bestName := ""
	for _, candidate := range candidates {
		idx := strings.Index(command, candidate)
		if idx >= 0 && (bestIndex < 0 || idx < bestIndex) {
			bestIndex = idx
			bestName = candidate
		}
	}
	return bestIndex, bestName
}

func TestNewRedisClientReturnsConstructorError(t *testing.T) {
	instrumentErr := errors.New("instrumentation failed")
	lc := fxtest.NewLifecycle(t)

	client, err := NewRedisClient(
		lc,
		zap.NewNop(),
		testCacheRedis,
		resources.RedisConfig{Mode: resources.RedisModeCluster, Addrs: []string{"127.0.0.1:6379"}},
		withRedisInstrumenterForTest(func(redis.UniversalClient, trace.TracerProvider) error {
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
