package datastore

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
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
)

var testDriverSeq atomic.Int64

const testAuditDB = "audit_db"
const testUserDB = "user_db"
const testCacheRedis = "cache_redis"

func TestNewPostgresReturnsErrorForMissingConfig(t *testing.T) {
	cfg := &config.Config{}
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := NewPostgres(lc, cfg, log, "missing_db")
	require.Error(t, err)
	require.Contains(t, err.Error(), `postgres config "missing_db" not found`)
}

func TestNewPostgresAppliesPoolSettings(t *testing.T) {
	drv := registerTestSQLDriver(t)
	cfg := testConfig(drv.name)
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	db, err := NewPostgres(lc, cfg, log, testUserDB)
	require.NoError(t, err)
	defer db.Close()

	stats := db.Stats()
	require.Equal(t, 7, stats.MaxOpenConnections)
}

func TestNewPostgresRegistersLifecycle(t *testing.T) {
	drv := registerTestSQLDriver(t)
	cfg := testConfig(drv.name)
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := NewPostgres(lc, cfg, log, testUserDB)
	require.NoError(t, err)
	lc.RequireStart()
	lc.RequireStop()

	require.Equal(t, int64(1), drv.pings.Load())
	require.Equal(t, int64(1), drv.closes.Load())
}

func TestNewPostgresClosesPoolWhenStartPingFails(t *testing.T) {
	drv := registerTestSQLDriver(t)
	drv.pingErr = errors.New("postgres unavailable")
	drv.closeErr = errors.New("driver close failed")
	cfg := testConfig(drv.name)
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	db, err := NewPostgres(lc, cfg, log, testUserDB)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Start(ctx)
	require.ErrorContains(t, err, "ping postgres user_db: postgres unavailable")
	require.ErrorContains(t, err, "close postgres user_db: driver close failed")
	require.Equal(t, int64(1), drv.pings.Load())
	require.Equal(t, int64(1), drv.closes.Load())
	require.ErrorContains(t, db.PingContext(ctx), "database is closed")
}

func TestNewPostgresPoolsRegistersSingleLifecycleForDeclaredPools(t *testing.T) {
	drv := registerTestSQLDriver(t)
	cfg := testConfig(drv.name)
	cfg.Postgres[testAuditDB] = cfg.Postgres[testUserDB]
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	dbs, err := NewPostgresPools(lc, cfg, log, testUserDB, testAuditDB)
	require.NoError(t, err)
	require.NotNil(t, dbs[testUserDB])
	require.NotNil(t, dbs[testAuditDB])
	lc.RequireStart()
	lc.RequireStop()

	require.Equal(t, int64(2), drv.pings.Load())
	require.Equal(t, int64(2), drv.closes.Load())
}

func TestNewPostgresPoolsStopPreservesNamedCloseErrors(t *testing.T) {
	drv := registerTestSQLDriver(t)
	drv.closeErr = errors.New("driver close failed")
	cfg := testConfig(drv.name)
	cfg.Postgres[testAuditDB] = cfg.Postgres[testUserDB]
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := NewPostgresPools(lc, cfg, log, testUserDB, testAuditDB)
	require.NoError(t, err)
	lc.RequireStart()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Stop(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "close postgres user_db")
	require.Contains(t, err.Error(), "close postgres audit_db")
	require.Equal(t, int64(2), drv.closes.Load())
}

func TestExplicitCommonProvidersDoNotProvideNamedPostgresPools(t *testing.T) {
	type params struct {
		fx.In

		UserDB *sql.DB `name:"user_db"`
	}

	err := fx.ValidateApp(
		fx.Supply(config.ConfigPath("config.yaml")),
		fx.Provide(config.NewConfig, logger.NewLogger),
		fx.Invoke(func(params) {}),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), `name="user_db"`)
}

func TestNewRedisClientReturnsErrorForMissingConfig(t *testing.T) {
	cfg := &config.Config{}
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := NewRedisClient(lc, cfg, log, "missing_redis")
	require.Error(t, err)
	require.Contains(t, err.Error(), `redis config "missing_redis" not found`)
}

func TestNewRedisClientRegistersLifecycle(t *testing.T) {
	redisServer := newTestRedisServer(t)
	cfg := &config.Config{Redis: map[string]config.RedisConfig{
		testCacheRedis: {
			Addr:         redisServer.addr,
			DB:           0,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			PingTimeout:  time.Second,
		},
	}}
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	client, err := NewRedisClient(lc, cfg, log, testCacheRedis)
	require.NoError(t, err)
	lc.RequireStart()
	lc.RequireStop()

	require.Equal(t, redisServer.addr, client.Options().Addr)
	require.Equal(t, int64(1), redisServer.pings.Load())
}

func TestNewRedisClientClosesClientWhenStartPingFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	cfg := &config.Config{Redis: map[string]config.RedisConfig{
		testCacheRedis: {
			Addr:         addr,
			DB:           0,
			DialTimeout:  10 * time.Millisecond,
			ReadTimeout:  10 * time.Millisecond,
			WriteTimeout: 10 * time.Millisecond,
			PingTimeout:  50 * time.Millisecond,
		},
	}}
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	client, err := NewRedisClient(lc, cfg, log, testCacheRedis)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Start(ctx)
	require.ErrorContains(t, err, "ping redis cache_redis")
	require.ErrorIs(t, client.Ping(ctx).Err(), redis.ErrClosed)
}

func TestOpenRedisClientCreatesTracingSpanWithExplicitProvider(t *testing.T) {
	redisServer := newTestRedisServer(t)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(noop.NewTracerProvider())
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	client := OpenRedisClient(config.RedisConfig{
		Addr:         redisServer.addr,
		DB:           0,
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	}, WithRedisTracerProvider(provider))
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx, parent := provider.Tracer("datastore-test").Start(context.Background(), "parent")
	require.NoError(t, client.Ping(ctx).Err())
	parent.End()

	spans := recorder.Ended()
	require.GreaterOrEqual(t, len(spans), 2)
	require.True(t, hasRedisSpan(spans), "spans=%v", spanNames(spans))
	require.Equal(t, int64(1), redisServer.pings.Load())
}

func TestOpenRedisClientUsesGlobalTracerProviderByDefault(t *testing.T) {
	redisServer := newTestRedisServer(t)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	client := OpenRedisClient(config.RedisConfig{
		Addr:         redisServer.addr,
		DB:           0,
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	})
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	ctx, parent := provider.Tracer("datastore-global-test").Start(context.Background(), "parent")
	require.NoError(t, client.Ping(ctx).Err())
	parent.End()

	require.Equal(t, int64(1), redisServer.pings.Load())
	require.True(t, hasRedisSpan(recorder.Ended()), "spans=%v", spanNames(recorder.Ended()))
}

func TestOpenRedisClientWorksWithNoopTracing(t *testing.T) {
	redisServer := newTestRedisServer(t)
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(noop.NewTracerProvider())
	t.Cleanup(func() { otel.SetTracerProvider(previousProvider) })

	client := OpenRedisClient(config.RedisConfig{
		Addr:         redisServer.addr,
		DB:           0,
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	})
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	require.NoError(t, client.Ping(context.Background()).Err())
	require.Equal(t, int64(1), redisServer.pings.Load())
}

func TestExplicitCommonProvidersDoNotProvideRedisClient(t *testing.T) {
	type params struct {
		fx.In

		Redis any `name:"cache_redis"`
	}

	err := fx.ValidateApp(
		fx.Supply(config.ConfigPath("config.yaml")),
		fx.Provide(config.NewConfig, logger.NewLogger),
		fx.Invoke(func(params) {}),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), `name="cache_redis"`)
}

func hasRedisSpan(spans []sdktrace.ReadOnlySpan) bool {
	for _, span := range spans {
		if span.Name() == "ping" || span.Name() == "redis.dial" {
			return true
		}
	}
	return false
}

func spanNames(spans []sdktrace.ReadOnlySpan) []string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name())
	}
	return names
}

func TestProvideNamedPostgresProvidesOnlyDeclaredPool(t *testing.T) {
	drv := registerTestSQLDriver(t)
	cfg := testConfig(drv.name)
	log := zap.NewNop()
	type params struct {
		fx.In

		UserDB *sql.DB `name:"user_db"`
	}
	var got params

	app := fxtest.New(t,
		fx.Supply(cfg, log),
		ProvideNamedPostgres(testUserDB, testUserDB),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, got.UserDB)
	require.Equal(t, int64(1), drv.pings.Load())
}

func TestProvideNamedRedisProvidesOnlyDeclaredClient(t *testing.T) {
	redisServer := newTestRedisServer(t)
	cfg := &config.Config{Redis: map[string]config.RedisConfig{
		testCacheRedis: {
			Addr:         redisServer.addr,
			DB:           0,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			PingTimeout:  time.Second,
		},
		"queue_redis": {
			Addr:         "127.0.0.1:1",
			DB:           1,
			DialTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			PingTimeout:  time.Second,
		},
	}}
	log := zap.NewNop()
	type params struct {
		fx.In

		CacheRedis *redis.Client `name:"cache_redis"`
	}
	var got params

	app := fxtest.New(t,
		fx.Supply(cfg, log),
		ProvideNamedRedis(testCacheRedis, testCacheRedis),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, got.CacheRedis)
	require.Equal(t, int64(1), redisServer.pings.Load())
}

func testConfig(driverName string) *config.Config {
	return &config.Config{
		Postgres: map[string]config.PostgresConfig{
			testUserDB: {
				Host:            "127.0.0.1",
				Port:            15432,
				Username:        "aegiscore",
				Password:        "secret",
				DBName:          "aegiscore_user",
				Driver:          driverName,
				MaxOpenConns:    7,
				MaxIdleConns:    3,
				ConnMaxLifetime: time.Minute,
				ConnMaxIdleTime: 30 * time.Second,
				PingTimeout:     time.Second,
			},
		},
	}
}

func newTestRedisServer(t *testing.T) *testRedisServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &testRedisServer{addr: listener.Addr().String()}
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

type testRedisServer struct {
	addr  string
	pings atomic.Int64
}

func (s *testRedisServer) handle(conn net.Conn) {
	defer conn.Close()
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

func (s *testRedisServer) writeResponses(w io.Writer, command string) error {
	responded := false
	for {
		idx, name := nextRedisCommand(command)
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

func nextRedisCommand(command string) (int, string) {
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

func registerTestSQLDriver(t *testing.T) *testSQLDriver {
	t.Helper()
	drv := &testSQLDriver{name: fmt.Sprintf("aegiscore_test_postgres_%d", testDriverSeq.Add(1))}
	sql.Register(drv.name, drv)
	return drv

}

type testSQLDriver struct {
	name   string
	pings  atomic.Int64
	closes atomic.Int64

	pingErr  error
	closeErr error
}

func (d *testSQLDriver) Open(string) (driver.Conn, error) {
	return &testSQLConn{driver: d}, nil
}

type testSQLConn struct {
	driver *testSQLDriver
}

func (c *testSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}

func (c *testSQLConn) Close() error {
	c.driver.closes.Add(1)
	return c.driver.closeErr
}

func (c *testSQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not implemented")
}

func (c *testSQLConn) Ping(context.Context) error {
	c.driver.pings.Add(1)
	return c.driver.pingErr
}
