package datastore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
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
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/runtime/resources"
)

var testDriverSeq atomic.Int64

const testAuditDB = "audit_db"
const testUserDB = "user_db"
const testCacheRedis = "cache_redis"

func TestNewPostgresReturnsErrorForMissingConfig(t *testing.T) {
	cfg := resources.PostgresConfigs{}
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := NewPostgres(lc, cfg, log, "missing_db")
	require.Error(t, err)
	require.Contains(t, err.Error(), `postgres config "missing_db" not found`)
}

func TestOpenPostgresAppliesPoolSettings(t *testing.T) {
	cfg := testPostgresConfig()
	db, err := OpenPostgres(testUserDB, cfg[testUserDB])
	require.NoError(t, err)
	defer db.Close()

	stats := db.Stats()
	require.Equal(t, 7, stats.MaxOpenConnections)
}

func TestOpenPostgresAppliesDefaultPoolSettings(t *testing.T) {
	db, err := OpenPostgres(testUserDB, resources.PostgresConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		Username: "aegiscore",
		DBName:   "aegiscore_user",
	})
	require.NoError(t, err)
	defer db.Close()

	require.Equal(t, resources.DefaultPostgresMaxOpenConns, db.Stats().MaxOpenConnections)
}

func TestNewPostgresPoolsAppliesDefaultsBeforeOpen(t *testing.T) {
	drv := registerTestSQLDriver(t)
	configs := resources.PostgresConfigs{
		testUserDB: {
			Host:     "127.0.0.1",
			Port:     5432,
			Username: "aegiscore",
			DBName:   "aegiscore_user",
		},
	}
	var opened resources.PostgresConfig
	opener := func(_ string, cfg resources.PostgresConfig) (*sql.DB, error) {
		opened = cfg
		return sql.Open(drv.name, "test")
	}

	dbs, err := newPostgresPools(fxtest.NewLifecycle(t), configs, zap.NewNop(), opener, testUserDB)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dbs[testUserDB].Close()) })
	require.Equal(t, resources.DefaultPostgresSSLMode, opened.SSLMode)
	require.Equal(t, resources.DefaultPostgresMaxOpenConns, opened.Pool.MaxOpenConns)
	require.Equal(t, resources.DefaultPostgresMaxIdleConns, opened.Pool.MaxIdleConns)
	require.Equal(t, resources.DefaultPostgresConnMaxLifetime, opened.Pool.ConnMaxLifetime)
	require.Equal(t, resources.DefaultPostgresConnMaxIdleTime, opened.Pool.ConnMaxIdleTime)
}

func TestPostgresDSNIncludesEscapedCredentialsDatabaseAndSSLMode(t *testing.T) {
	dsn := PostgresDSN(resources.PostgresConfig{
		Host:     "2001:db8::1",
		Port:     5432,
		Username: "aegis user",
		Password: "p@ss:/word",
		DBName:   "user/data",
		SSLMode:  "verify-full",
	})
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Equal(t, "[2001:db8::1]:5432", parsed.Host)
	require.Equal(t, "aegis user", parsed.User.Username())
	password, ok := parsed.User.Password()
	require.True(t, ok)
	require.Equal(t, "p@ss:/word", password)
	require.Equal(t, "/user/data", parsed.Path)
	require.Equal(t, "/user%2Fdata", parsed.RawPath)
	require.Equal(t, "verify-full", parsed.Query().Get("sslmode"))
}

func TestPostgresDSNAppliesDefaultSSLMode(t *testing.T) {
	dsn := PostgresDSN(resources.PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		Username: "aegiscore",
		DBName:   "aegiscore",
	})
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Equal(t, resources.DefaultPostgresSSLMode, parsed.Query().Get("sslmode"))
}

func TestNewPostgresRegistersLifecycle(t *testing.T) {
	drv := registerTestSQLDriver(t)
	cfg := testPostgresConfig()
	lc := fxtest.NewLifecycle(t)
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)

	_, err := newPostgres(lc, cfg, log, testSQLPostgresOpener(drv.name), testUserDB)
	require.NoError(t, err)
	lc.RequireStart()
	lc.RequireStop()

	require.Equal(t, int64(1), drv.pings.Load())
	require.Equal(t, int64(1), drv.closes.Load())
	require.Greater(t, time.Duration(drv.pingTimeoutNanos.Load()), 4*time.Second)
	require.LessOrEqual(t, time.Duration(drv.pingTimeoutNanos.Load()), resources.DefaultPostgresPingTimeout())
	assertDatastoreLifecycleLog(t, logs, "postgres connected", "postgres", "postgres", testUserDB)
	assertDatastoreLifecycleLog(t, logs, "postgres closed", "postgres", "postgres", testUserDB)
}

func TestNewPostgresClosesPoolWhenStartPingFails(t *testing.T) {
	drv := registerTestSQLDriver(t)
	drv.pingErr = errors.New("postgres unavailable")
	drv.closeErr = errors.New("driver close failed")
	cfg := testPostgresConfig()
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	db, err := newPostgres(lc, cfg, log, testSQLPostgresOpener(drv.name), testUserDB)
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
	cfg := testPostgresConfig()
	cfg[testAuditDB] = cfg[testUserDB]
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	dbs, err := newPostgresPools(lc, cfg, log, testSQLPostgresOpener(drv.name), testUserDB, testAuditDB)
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
	cfg := testPostgresConfig()
	cfg[testAuditDB] = cfg[testUserDB]
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := newPostgresPools(lc, cfg, log, testSQLPostgresOpener(drv.name), testUserDB, testAuditDB)
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
	cfg := resources.RedisConfigs{}
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := NewRedisClient(lc, cfg, log, "missing_redis")
	require.Error(t, err)
	require.Contains(t, err.Error(), `redis config "missing_redis" not found`)
}

func TestNewRedisClientRegistersLifecycle(t *testing.T) {
	redisServer := newTestRedisServer(t)
	cfg := resources.RedisConfigs{
		testCacheRedis: {
			Addr: redisServer.addr,
			DB:   0,
		},
	}
	lc := fxtest.NewLifecycle(t)
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)

	client, err := NewRedisClient(lc, cfg, log, testCacheRedis)
	require.NoError(t, err)
	lc.RequireStart()
	lc.RequireStop()

	require.Equal(t, redisServer.addr, client.Options().Addr)
	require.Equal(t, resources.DefaultRedisTimeout, client.Options().DialTimeout)
	require.Equal(t, resources.DefaultRedisTimeout, client.Options().ReadTimeout)
	require.Equal(t, resources.DefaultRedisTimeout, client.Options().WriteTimeout)
	require.Equal(t, int64(1), redisServer.pings.Load())
	assertDatastoreLifecycleLog(t, logs, "redis connected", "redis", "redis", testCacheRedis)
	assertDatastoreLifecycleLog(t, logs, "redis closed", "redis", "redis", testCacheRedis)
}

func assertDatastoreLifecycleLog(t *testing.T, logs *observer.ObservedLogs, message string, loggerName string, component string, resource string) {
	t.Helper()
	entries := logs.FilterMessage(message).All()
	require.Len(t, entries, 1)
	require.Equal(t, loggerName, entries[0].LoggerName)
	fields := entries[0].ContextMap()
	require.Equal(t, component, fields[logger.ComponentField])
	require.Equal(t, resource, fields[logger.ResourceField])
}

func TestNewRedisClientClosesClientWhenStartPingFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	cfg := resources.RedisConfigs{
		testCacheRedis: {
			Addr:    addr,
			DB:      0,
			Timeout: 20 * time.Millisecond,
		},
	}
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

	client := OpenRedisClient(resources.RedisConfig{
		Addr:    redisServer.addr,
		DB:      0,
		Timeout: time.Second,
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
	client := OpenRedisClient(resources.RedisConfig{
		Addr:    redisServer.addr,
		DB:      0,
		Timeout: time.Second,
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

	client := OpenRedisClient(resources.RedisConfig{
		Addr:    redisServer.addr,
		DB:      0,
		Timeout: time.Second,
	})
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	require.NoError(t, client.Ping(context.Background()).Err())
	require.Equal(t, int64(1), redisServer.pings.Load())
}

func TestOpenRedisClientMapsDefaultTimeoutToAllOperations(t *testing.T) {
	client := OpenRedisClient(resources.RedisConfig{
		Addr:     "127.0.0.1:6379",
		Username: "service-account",
		Password: "secret",
	})
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	opts := client.Options()
	require.Equal(t, "service-account", opts.Username)
	require.Equal(t, "secret", opts.Password)
	require.Equal(t, resources.DefaultRedisTimeout, opts.DialTimeout)
	require.Equal(t, resources.DefaultRedisTimeout, opts.ReadTimeout)
	require.Equal(t, resources.DefaultRedisTimeout, opts.WriteTimeout)
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
	cfg := testPostgresConfig()
	log := zap.NewNop()
	type params struct {
		fx.In

		UserDB *sql.DB `name:"user_db"`
	}
	err := fx.ValidateApp(
		fx.Supply(cfg, log),
		ProvideNamedPostgres(testUserDB, testUserDB),
		fx.Invoke(func(params) {}),
	)
	require.NoError(t, err)
}

func TestProvideNamedRedisProvidesOnlyDeclaredClient(t *testing.T) {
	redisServer := newTestRedisServer(t)
	cfg := resources.RedisConfigs{
		testCacheRedis: {
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

func testPostgresConfig() resources.PostgresConfigs {
	return resources.PostgresConfigs{
		testUserDB: {
			Host:     "127.0.0.1",
			Port:     15432,
			Username: "aegiscore",
			Password: "secret",
			DBName:   "aegiscore_user",
			SSLMode:  "disable",
			Pool: resources.PostgresPoolConfig{
				MaxOpenConns:    7,
				MaxIdleConns:    3,
				ConnMaxLifetime: time.Minute,
				ConnMaxIdleTime: 30 * time.Second,
			},
		},
	}
}

func newPostgres(lc fx.Lifecycle, configs resources.PostgresConfigs, log *zap.Logger, opener postgresOpener, name string) (*sql.DB, error) {
	dbs, err := newPostgresPools(lc, configs, log, opener, name)
	if err != nil {
		return nil, err
	}
	return dbs[name], nil
}

func testSQLPostgresOpener(driverName string) postgresOpener {
	return func(_ string, cfg resources.PostgresConfig) (*sql.DB, error) {
		cfg.ApplyDefaults()
		db, err := sql.Open(driverName, "test")
		if err != nil {
			return nil, err
		}
		applyPostgresPoolConfig(db, cfg.Pool)
		return db, nil
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
	name             string
	pings            atomic.Int64
	closes           atomic.Int64
	pingTimeoutNanos atomic.Int64

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

func (c *testSQLConn) Ping(ctx context.Context) error {
	c.driver.pings.Add(1)
	if deadline, ok := ctx.Deadline(); ok {
		c.driver.pingTimeoutNanos.Store(int64(time.Until(deadline)))
	}
	return c.driver.pingErr
}
