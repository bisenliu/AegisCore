package providers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	commonresources "github.com/aegiscore/common/runtime/resources"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/resources"
)

var providerTestDriverSeq atomic.Int64

func TestProvidePostgresPoolsProvidesUserDatabase(t *testing.T) {
	cfg := providerTestConfig("")
	lc := fxtest.NewLifecycle(t)
	got, err := ProvidePostgresPools(NamedPostgresParams{Lifecycle: lc, Config: cfg, Log: zap.NewNop()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = got.UserDB.Close() })

	require.NotNil(t, got.UserDB)
	require.Equal(t, 7, got.UserDB.Stats().MaxOpenConnections)
}

func TestProvidePostgresPoolsReturnsErrorForMissingUserDatabase(t *testing.T) {
	_, err := ProvidePostgresPools(NamedPostgresParams{
		Lifecycle: fxtest.NewLifecycle(t),
		Config:    &serviceconfig.Config{},
		Log:       zap.NewNop(),
	})
	require.ErrorContains(t, err, `postgres config "`+resources.NameUserDB+`" not found`)
}

func TestProvidePostgresPoolsDoesNotProvideSharedDatabase(t *testing.T) {
	type pools struct {
		fx.In

		SharedDB *sql.DB `name:"shared_db"`
	}

	err := fx.ValidateApp(
		fx.Provide(ProvidePostgresPools),
		fx.Invoke(func(pools) {}),
	)
	require.ErrorContains(t, err, `name="shared_db"`)
}

func TestProvidePostgresPoolsDoesNotProvidePayDatabase(t *testing.T) {
	type pools struct {
		fx.In

		PayDB *sql.DB `name:"pay_db"`
	}

	err := fx.ValidateApp(
		fx.Provide(ProvidePostgresPools),
		fx.Invoke(func(pools) {}),
	)
	require.ErrorContains(t, err, `name="pay_db"`)
}

func TestProvideEntClientsProvidesUserServiceEntClient(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	userDB, err := sql.Open(drv.name, "postgres://aegiscore:secret@127.0.0.1/aegiscore_user")
	require.NoError(t, err)
	defer userDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, userDB.PingContext(ctx))

	lc := fxtest.NewLifecycle(t)
	got, err := ProvideEntClients(NamedEntClientParams{
		Lifecycle: lc,
		Log:       zap.NewNop(),
		UserDB:    userDB,
	})
	require.NoError(t, err)

	require.NotNil(t, got.UserClient)
	require.Equal(t, int64(0), drv.closes.Load())

	require.NoError(t, lc.Start(ctx))
	require.NoError(t, lc.Stop(ctx))
	require.Equal(t, int64(0), drv.closes.Load())
}

func TestProvideRedisClientsProvidesCacheRedis(t *testing.T) {
	redisServer := newProviderTestRedisServer(t)
	traceProvider := newProviderTestTracing(t)
	spanRecorder := tracetest.NewSpanRecorder()
	traceProvider.TracerProvider().RegisterSpanProcessor(spanRecorder)
	cfg := providerTestConfig("")
	cfg.Resources.Redis = commonresources.RedisConfigs{
		resources.NameCacheRedis: {
			Addr:    redisServer.addr,
			DB:      0,
			Timeout: time.Second,
		},
		"queue_redis": {
			Addr:    "127.0.0.1:1",
			DB:      1,
			Timeout: time.Second,
		},
	}
	log := zap.NewNop()

	type clients struct {
		fx.In

		CacheRedis *redis.Client `name:"cache_redis"`
	}

	var got clients
	app := fxtest.New(t,
		fx.Supply(cfg, log, traceProvider),
		fx.Provide(ProvideRedisClients),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, got.CacheRedis)
	require.Equal(t, redisServer.addr, got.CacheRedis.Options().Addr)
	require.Equal(t, int64(1), redisServer.pings.Load())
	require.True(t, providerTestHasRedisSpan(spanRecorder), "spans=%v", providerTestSpanNames(spanRecorder))
	redisServer.requireClosed(t)
}

func TestProvideRedisClientsDoesNotProvideQueueRedis(t *testing.T) {
	traceProvider := newProviderTestTracing(t)
	type clients struct {
		fx.In

		QueueRedis *redis.Client `name:"queue_redis"`
	}

	err := fx.ValidateApp(
		fx.Supply(&serviceconfig.Config{}, zap.NewNop(), traceProvider),
		fx.Provide(ProvideRedisClients),
		fx.Invoke(func(clients) {}),
	)
	require.ErrorContains(t, err, `name="queue_redis"`)
}

func TestProvideRedisClientsReturnsErrorForMissingCacheRedisConfig(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()
	traceProvider := newProviderTestTracing(t)

	_, err := ProvideRedisClients(NamedRedisParams{
		Lifecycle: lc,
		Config:    &serviceconfig.Config{},
		Log:       log,
		Trace:     traceProvider,
	})
	require.ErrorContains(t, err, `redis config "`+resources.NameCacheRedis+`" not found`)
}

func TestProvideRedisClientsRequiresTracingProvider(t *testing.T) {
	_, err := ProvideRedisClients(NamedRedisParams{
		Lifecycle: fxtest.NewLifecycle(t),
		Config:    providerTestConfig(""),
		Log:       zap.NewNop(),
	})
	require.ErrorContains(t, err, "redis tracing provider is required")
}

func TestProvideRedisClientsFailsStartWhenCacheRedisUnavailable(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()
	traceProvider := newProviderTestTracing(t)
	cfg := &serviceconfig.Config{Resources: serviceconfig.ResourcesConfig{Redis: commonresources.RedisConfigs{
		resources.NameCacheRedis: {Addr: "127.0.0.1:1", DB: 0, Timeout: 10 * time.Millisecond},
	}}}

	clients, err := ProvideRedisClients(NamedRedisParams{Lifecycle: lc, Config: cfg, Log: log, Trace: traceProvider})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Start(ctx)
	require.ErrorContains(t, err, "ping redis cache_redis")
	require.ErrorIs(t, clients.CacheRedis.Ping(ctx).Err(), redis.ErrClosed)
}

func newProviderTestTracing(t *testing.T) *commontracing.Provider {
	t.Helper()
	cfg := &config.Config{
		App: config.AppConfig{Name: "provider-test", Environment: "test"},
		Observability: config.ObservabilityConfig{Tracing: config.TracingConfig{
			Enabled:      true,
			SampleRatio:  1,
			OTLPEndpoint: "127.0.0.1:4317",
			Insecure:     true,
		}},
	}
	return newGinTestTracingProvider(t, cfg)
}

func providerTestHasRedisSpan(recorder *tracetest.SpanRecorder) bool {
	for _, span := range recorder.Ended() {
		if span.Name() == "ping" || span.Name() == "redis.dial" {
			return true
		}
	}
	return false
}

func providerTestSpanNames(recorder *tracetest.SpanRecorder) []string {
	spans := recorder.Ended()
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name())
	}
	return names
}

func providerTestConfig(_ string) *serviceconfig.Config {
	return &serviceconfig.Config{
		Resources: serviceconfig.ResourcesConfig{
			Redis: commonresources.RedisConfigs{
				resources.NameCacheRedis: {
					Addr:    "127.0.0.1:6379",
					DB:      0,
					Timeout: time.Second,
				},
			},
			Postgres: commonresources.PostgresConfigs{
				resources.NameUserDB: {
					Host: "127.0.0.1", Port: 15432, Username: "aegiscore", Password: "secret", DBName: "aegiscore_user", SSLMode: "disable",
					Pool: commonresources.PostgresPoolConfig{MaxOpenConns: 7, MaxIdleConns: 3, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: 30 * time.Second},
				},
				"pay_db": {
					Host: "127.0.0.1", Port: 15432, Username: "aegiscore", Password: "secret", DBName: "aegiscore_pay", SSLMode: "disable",
					Pool: commonresources.PostgresPoolConfig{MaxOpenConns: 99, MaxIdleConns: 3, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: 30 * time.Second},
				},
			},
		},
	}
}

func newProviderTestRedisServer(t *testing.T) *providerTestRedisServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &providerTestRedisServer{
		addr:   listener.Addr().String(),
		closed: make(chan struct{}, 1),
	}
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

type providerTestRedisServer struct {
	addr   string
	pings  atomic.Int64
	closed chan struct{}
}

func (s *providerTestRedisServer) handle(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		select {
		case s.closed <- struct{}{}:
		default:
		}
	}()

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

func (s *providerTestRedisServer) writeResponses(w io.Writer, command string) error {
	responded := false
	for {
		idx, name := nextProviderRedisCommand(command)
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
		case "PING":
			s.pings.Add(1)
			if _, err := w.Write([]byte("+PONG\r\n")); err != nil {
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

func nextProviderRedisCommand(command string) (int, string) {
	candidates := []string{"PING", "HELLO", "CLIENT"}
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

func (s *providerTestRedisServer) requireClosed(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case <-s.closed:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
}

func registerProviderTestSQLDriver(t *testing.T) *providerTestSQLDriver {
	t.Helper()
	drv := &providerTestSQLDriver{name: fmt.Sprintf("aegiscore_provider_test_postgres_%d", providerTestDriverSeq.Add(1))}
	sql.Register(drv.name, drv)
	return drv
}

type providerTestSQLDriver struct {
	name   string
	pings  atomic.Int64
	closes atomic.Int64
}

func (d *providerTestSQLDriver) Open(string) (driver.Conn, error) {
	return &providerTestSQLConn{driver: d}, nil
}

type providerTestSQLConn struct {
	driver *providerTestSQLDriver
}

func (c *providerTestSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}

func (c *providerTestSQLConn) Close() error {
	c.driver.closes.Add(1)
	return nil
}

func (c *providerTestSQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not implemented")
}

func (c *providerTestSQLConn) Ping(context.Context) error {
	c.driver.pings.Add(1)
	return nil
}
