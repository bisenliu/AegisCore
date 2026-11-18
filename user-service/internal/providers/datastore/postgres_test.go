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
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastore"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	commonresources "github.com/aegiscore/common/runtime/resources"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	"github.com/aegiscore/user-service/internal/resources"
)

var providerTestDriverSeq atomic.Int64

func TestProvidePostgresProvidesNamedDatabase(t *testing.T) {
	cfg := providerTestConfig("")
	lc := fxtest.NewLifecycle(t)
	got, err := ProvidePostgres(resources.NamePrimaryDB)(PostgresParams{Lifecycle: lc, Settings: cfg, Log: zap.NewNop(), Trace: newProviderTestDisabledTracing(t)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = got.Close() })

	require.NotNil(t, got)
	require.Equal(t, 7, got.Stats().MaxOpenConnections)
}

func TestProvidePostgresProvidesCustomResourceName(t *testing.T) {
	cfg := serviceconfig.ResourceSettings{Postgres: commonresources.PostgresConfigs{
		"analytics_db": {
			Host: "127.0.0.1", Port: 15432, Username: "aegiscore", Password: "secret", DBName: "analytics", SSLMode: "disable",
			Pool: commonresources.PostgresPoolConfig{MaxOpenConns: 4, MaxIdleConns: 1, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: 30 * time.Second},
		},
	}}
	got, err := ProvidePostgres("analytics_db")(PostgresParams{Lifecycle: fxtest.NewLifecycle(t), Settings: cfg, Log: zap.NewNop(), Trace: newProviderTestDisabledTracing(t)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = got.Close() })

	require.Equal(t, 4, got.Stats().MaxOpenConnections)
}

func TestProvidePostgresReturnsErrorForMissingResource(t *testing.T) {
	_, err := ProvidePostgres("analytics_db")(PostgresParams{
		Lifecycle: fxtest.NewLifecycle(t),
		Settings:  serviceconfig.ResourceSettings{},
		Log:       zap.NewNop(),
		Trace:     newProviderTestDisabledTracing(t),
	})
	require.ErrorContains(t, err, `postgres config "analytics_db" not found`)
}

func TestProvidePostgresRequiresResourceName(t *testing.T) {
	_, err := ProvidePostgres("")(PostgresParams{Lifecycle: fxtest.NewLifecycle(t), Settings: providerTestConfig(""), Log: zap.NewNop(), Trace: newProviderTestDisabledTracing(t)})
	require.ErrorContains(t, err, "postgres resource name is required")
}

func TestProvidePostgresRequiresTracingProvider(t *testing.T) {
	_, err := ProvidePostgres(resources.NamePrimaryDB)(PostgresParams{Lifecycle: fxtest.NewLifecycle(t), Settings: providerTestConfig(""), Log: zap.NewNop()})
	require.ErrorContains(t, err, "postgres tracing provider is required")
}

func TestProvidePostgresDoesNotProvideUndeclaredDatabase(t *testing.T) {
	type pools struct {
		fx.In

		PayDB *sql.DB `name:"pay_db"`
	}

	err := fx.ValidateApp(
		fx.Provide(fx.Annotate(ProvidePostgres(resources.NamePrimaryDB), fx.ResultTags(`name:"primary_db"`))),
		fx.Invoke(func(pools) {}),
	)
	require.ErrorContains(t, err, `name="pay_db"`)
}

func TestProvideEntClientProvidesNamedClient(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	userDB, err := sql.Open(drv.name, "postgres://aegiscore:secret@127.0.0.1/aegiscore_user")
	require.NoError(t, err)
	defer userDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, userDB.PingContext(ctx))

	lc := fxtest.NewLifecycle(t)
	plugins, err := ProvideEntPlugins(serviceconfig.EntSettings{}, zap.NewNop(), newProviderTestMetrics(t, true), newProviderTestTracing(t))
	require.NoError(t, err)
	got, err := ProvideEntClient("analytics_db")(lc, zap.NewNop(), userDB, plugins)
	require.NoError(t, err)

	require.NotNil(t, got)
	require.Equal(t, int64(0), drv.closes.Load())

	require.NoError(t, lc.Start(ctx))
	require.NoError(t, lc.Stop(ctx))
	require.Equal(t, int64(0), drv.closes.Load())
}

func TestProvideEntClientAcceptsDisabledObservabilityProviders(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	userDB, err := sql.Open(drv.name, "postgres://aegiscore:secret@127.0.0.1/aegiscore_user")
	require.NoError(t, err)
	defer userDB.Close()

	plugins, err := ProvideEntPlugins(serviceconfig.EntSettings{}, zap.NewNop(), newProviderTestMetrics(t, false), newProviderTestDisabledTracing(t))
	require.NoError(t, err)
	got, err := ProvideEntClient(resources.NamePrimaryDB)(fxtest.NewLifecycle(t), zap.NewNop(), userDB, plugins)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestProvideEntPluginsRequiresMetricsProvider(t *testing.T) {
	_, err := ProvideEntPlugins(serviceconfig.EntSettings{}, zap.NewNop(), nil, newProviderTestDisabledTracing(t))
	require.ErrorContains(t, err, "ent metrics provider is required")
}

func TestProvideEntPluginsRequiresTracingProvider(t *testing.T) {
	_, err := ProvideEntPlugins(serviceconfig.EntSettings{}, zap.NewNop(), newProviderTestMetrics(t, false), nil)
	require.ErrorContains(t, err, "ent tracing provider is required")
}

func TestProvideEntClientRequiresResourceName(t *testing.T) {
	_, err := ProvideEntClient("")(fxtest.NewLifecycle(t), zap.NewNop(), nil, EntPlugins{})
	require.ErrorContains(t, err, "ent resource name is required")
}

func TestProvideEntClientResolvesWithObservabilityProvidersInGraph(t *testing.T) {
	type clients struct {
		fx.In

		Client *ent.Client `name:"analytics_db"`
	}

	err := fx.ValidateApp(
		fx.Supply(serviceconfig.EntSettings{}, zap.NewNop(), newProviderTestMetrics(t, false), newProviderTestDisabledTracing(t)),
		fx.Provide(
			fx.Annotate(provideSQLDBForEntGraphTest, fx.ResultTags(`name:"analytics_db"`)),
			ProvideEntPlugins,
			fx.Annotate(ProvideEntClient("analytics_db"), fx.ParamTags("", "", `name:"analytics_db"`), fx.ResultTags(`name:"analytics_db"`)),
		),
		fx.Invoke(func(clients) {}),
	)
	require.NoError(t, err)
}

func TestProvideRedisProvidesNamedClient(t *testing.T) {
	redisServer := newProviderTestRedisServer(t)
	traceCfg := &config.Config{App: config.AppConfig{Name: "provider-test", Environment: "test"}, Observability: config.ObservabilityConfig{Tracing: config.TracingConfig{Enabled: true, SampleRatio: 1}}}
	traceProvider, traceCollector := newGinTestTracingProviderWithCollector(t, traceCfg)
	cfg := providerTestConfig("")
	cfg.Redis = commonresources.RedisConfigs{
		resources.NameCacheRedis: {
			Mode:    commonresources.RedisModeCluster,
			Addrs:   []string{redisServer.addr},
			Timeout: time.Second,
		},
		"queue_redis": {
			Mode:    commonresources.RedisModeCluster,
			Addrs:   []string{"127.0.0.1:1"},
			Timeout: time.Second,
		},
	}
	log := zap.NewNop()

	type clients struct {
		fx.In

		CacheRedis redis.UniversalClient `name:"cache_redis"`
	}

	var got clients
	app := fxtest.New(t,
		fx.Supply(cfg, log, traceProvider),
		fx.Provide(fx.Annotate(ProvideRedis(resources.NameCacheRedis), fx.ResultTags(`name:"cache_redis"`))),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, got.CacheRedis)
	require.Equal(t, int64(1), redisServer.pings.Load())
	spans := exportedSpans(t, traceProvider, traceCollector)
	require.True(t, providerTestHasRedisSpan(spans), "spans=%v", providerTestSpanNames(spans))
	redisServer.requireClosed(t)
}

func TestProvideRedisDoesNotProvideUndeclaredClient(t *testing.T) {
	traceProvider := newProviderTestTracing(t)
	type clients struct {
		fx.In

		QueueRedis redis.UniversalClient `name:"queue_redis"`
	}

	err := fx.ValidateApp(
		fx.Supply(serviceconfig.ResourceSettings{}, zap.NewNop(), traceProvider),
		fx.Provide(fx.Annotate(ProvideRedis(resources.NameCacheRedis), fx.ResultTags(`name:"cache_redis"`))),
		fx.Invoke(func(clients) {}),
	)
	require.ErrorContains(t, err, `name="queue_redis"`)
}

func TestProvideRedisProvidesCustomResourceName(t *testing.T) {
	redisServer := newProviderTestRedisServer(t)
	cfg := serviceconfig.ResourceSettings{Redis: commonresources.RedisConfigs{
		"queue_redis": {Mode: commonresources.RedisModeCluster, Addrs: []string{redisServer.addr}, Timeout: time.Second},
	}}

	client, err := ProvideRedis("queue_redis")(RedisParams{
		Lifecycle: fxtest.NewLifecycle(t),
		Settings:  cfg,
		Log:       zap.NewNop(),
		Trace:     newProviderTestTracing(t),
	})
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.Close())
}

func TestProvideRedisReturnsErrorForMissingResource(t *testing.T) {
	_, err := ProvideRedis("queue_redis")(RedisParams{
		Lifecycle: fxtest.NewLifecycle(t),
		Settings:  serviceconfig.ResourceSettings{},
		Log:       zap.NewNop(),
		Trace:     newProviderTestTracing(t),
	})
	require.ErrorContains(t, err, `redis config "queue_redis" not found`)
}

func TestProvideRedisRequiresResourceName(t *testing.T) {
	_, err := ProvideRedis("")(RedisParams{
		Lifecycle: fxtest.NewLifecycle(t),
		Settings:  providerTestConfig(""),
		Log:       zap.NewNop(),
		Trace:     newProviderTestTracing(t),
	})
	require.ErrorContains(t, err, "redis resource name is required")
}

func TestProvideRedisRequiresTracingProvider(t *testing.T) {
	_, err := ProvideRedis(resources.NameCacheRedis)(RedisParams{
		Lifecycle: fxtest.NewLifecycle(t),
		Settings:  providerTestConfig(""),
		Log:       zap.NewNop(),
	})
	require.ErrorContains(t, err, "redis tracing provider is required")
}

func TestProvideRedisAcceptsUnstartedTracingProvider(t *testing.T) {
	client, err := ProvideRedis(resources.NameCacheRedis)(RedisParams{
		Lifecycle: fxtest.NewLifecycle(t),
		Settings:  providerTestConfig(""),
		Log:       zap.NewNop(),
		Trace:     &commontracing.Provider{},
	})
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.Close())
}

func TestProvideRedisUsesFxTracingProviderDuringConstruction(t *testing.T) {
	redisServer := newProviderTestRedisServer(t)
	cfg := providerTestConfig("")
	runtimeCfg := &config.Config{
		App:           config.AppConfig{Name: "provider-test", Environment: "test"},
		Observability: config.ObservabilityConfig{Tracing: config.TracingConfig{Enabled: false, SampleRatio: 1}},
	}
	cfg.Redis[resources.NameCacheRedis] = commonresources.RedisConfig{Mode: commonresources.RedisModeCluster, Addrs: []string{redisServer.addr}, Timeout: time.Second}

	type clients struct {
		fx.In

		CacheRedis redis.UniversalClient `name:"cache_redis"`
	}
	var got clients
	app := fxtest.New(t,
		fx.Supply(cfg, runtimeCfg, zap.NewNop()),
		fx.Provide(commontracing.NewTracingProvider),
		fx.Provide(fx.Annotate(ProvideRedis(resources.NameCacheRedis), fx.ResultTags(`name:"cache_redis"`))),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, got.CacheRedis)
}

func TestProvideRedisWrapsConstructorError(t *testing.T) {
	constructorErr := errors.New("instrumentation failed")
	traceProvider := newProviderTestTracing(t)

	client, err := newRedis(resources.NameCacheRedis, RedisParams{
		Lifecycle: fxtest.NewLifecycle(t),
		Settings:  providerTestConfig(""),
		Log:       zap.NewNop(),
		Trace:     traceProvider,
	}, func(fx.Lifecycle, *zap.Logger, string, commonresources.RedisConfig, ...datastore.RedisClientOption) (redis.UniversalClient, error) {
		return nil, constructorErr
	})
	require.Nil(t, client)
	require.ErrorContains(t, err, "provide redis cache_redis")
	require.ErrorIs(t, err, constructorErr)
}

func TestProvideRedisFailsStartWhenResourceUnavailable(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()
	traceProvider := newProviderTestTracing(t)
	cfg := serviceconfig.ResourceSettings{Redis: commonresources.RedisConfigs{
		resources.NameCacheRedis: {Mode: commonresources.RedisModeCluster, Addrs: []string{"127.0.0.1:1"}, Timeout: 10 * time.Millisecond},
	}}

	client, err := ProvideRedis(resources.NameCacheRedis)(RedisParams{Lifecycle: lc, Settings: cfg, Log: log, Trace: traceProvider})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Start(ctx)
	require.ErrorContains(t, err, "ping redis cache_redis")
	require.ErrorIs(t, client.Ping(ctx).Err(), redis.ErrClosed)
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

func newProviderTestDisabledTracing(t *testing.T) *commontracing.Provider {
	t.Helper()
	provider, err := commontracing.NewProvider(context.Background(), commontracing.Options{
		Config:      config.TracingConfig{Enabled: false, SampleRatio: 1},
		ServiceName: "provider-test",
		Environment: "test",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	return provider
}

func newProviderTestMetrics(t *testing.T, enabled bool) *commonmetrics.Provider {
	t.Helper()
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: enabled},
		ServiceName: "provider-test",
		Environment: "test",
	})
	require.NoError(t, err)
	return provider
}

func provideSQLDBForEntGraphTest() *sql.DB {
	return nil
}

func providerTestHasRedisSpan(spans []*tracepb.Span) bool {
	for _, span := range spans {
		if span.GetName() == "ping" || span.GetName() == "redis.dial" {
			return true
		}
	}
	return false
}

func providerTestSpanNames(spans []*tracepb.Span) []string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.GetName())
	}
	return names
}

func providerTestConfig(_ string) serviceconfig.ResourceSettings {
	return serviceconfig.ResourceSettings{
		Redis: commonresources.RedisConfigs{
			resources.NameCacheRedis: {
				Mode:    commonresources.RedisModeCluster,
				Addrs:   []string{"127.0.0.1:6379"},
				Timeout: time.Second,
			},
		},
		Postgres: commonresources.PostgresConfigs{
			resources.NamePrimaryDB: {
				Host: "127.0.0.1", Port: 15432, Username: "aegiscore", Password: "secret", DBName: "aegiscore_user", SSLMode: "disable",
				Pool: commonresources.PostgresPoolConfig{MaxOpenConns: 7, MaxIdleConns: 3, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: 30 * time.Second},
			},
			"pay_db": {
				Host: "127.0.0.1", Port: 15432, Username: "aegiscore", Password: "secret", DBName: "aegiscore_pay", SSLMode: "disable",
				Pool: commonresources.PostgresPoolConfig{MaxOpenConns: 99, MaxIdleConns: 3, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: 30 * time.Second},
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
		case "COMMAND":
			if _, err := w.Write([]byte("*0\r\n")); err != nil {
				return err
			}
		case "PING":
			s.pings.Add(1)
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

func nextProviderRedisCommand(command string) (int, string) {
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
