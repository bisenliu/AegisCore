package datastore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/runtime/resources"
)

var testDriverSeq atomic.Int64

const testAuditDB = "audit_db"

func TestNewPostgresReturnsErrorForMissingConfig(t *testing.T) {
	cfg := &config.Config{}
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := NewPostgres(lc, cfg, log, "missing_db")
	if err == nil {
		t.Fatal("NewPostgres error = nil")
	}
	if !strings.Contains(err.Error(), `postgres config "missing_db" not found`) {
		t.Fatalf("NewPostgres error = %q, want missing config", err.Error())
	}
}

func TestNewPostgresAppliesPoolSettings(t *testing.T) {
	drv := registerTestSQLDriver(t)
	cfg := testConfig(drv.name)
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	db, err := NewPostgres(lc, cfg, log, resources.NameUserDB)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer db.Close()

	stats := db.Stats()
	if stats.MaxOpenConnections != 7 {
		t.Fatalf("MaxOpenConnections = %d, want 7", stats.MaxOpenConnections)
	}
}

func TestNewPostgresRegistersLifecycle(t *testing.T) {
	drv := registerTestSQLDriver(t)
	cfg := testConfig(drv.name)
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	if _, err := NewPostgres(lc, cfg, log, resources.NameUserDB); err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	lc.RequireStart()
	lc.RequireStop()

	if got := drv.pings.Load(); got != 1 {
		t.Fatalf("pings = %d, want 1", got)
	}
	if got := drv.closes.Load(); got != 1 {
		t.Fatalf("closes = %d, want 1", got)
	}
}

func TestNewPostgresPoolsRegistersSingleLifecycleForDeclaredPools(t *testing.T) {
	drv := registerTestSQLDriver(t)
	cfg := testConfig(drv.name)
	cfg.Postgres[testAuditDB] = cfg.Postgres[resources.NameUserDB]
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	dbs, err := NewPostgresPools(lc, cfg, log, resources.NameUserDB, testAuditDB)
	if err != nil {
		t.Fatalf("NewPostgresPools: %v", err)
	}
	if dbs[resources.NameUserDB] == nil {
		t.Fatal("user_db = nil")
	}
	if dbs[testAuditDB] == nil {
		t.Fatal("audit_db = nil")
	}
	lc.RequireStart()
	lc.RequireStop()

	if got := drv.pings.Load(); got != 2 {
		t.Fatalf("pings = %d, want 2", got)
	}
	if got := drv.closes.Load(); got != 2 {
		t.Fatalf("closes = %d, want 2", got)
	}
}

func TestNewPostgresPoolsStopPreservesNamedCloseErrors(t *testing.T) {
	drv := registerTestSQLDriver(t)
	drv.closeErr = errors.New("driver close failed")
	cfg := testConfig(drv.name)
	cfg.Postgres[testAuditDB] = cfg.Postgres[resources.NameUserDB]
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	if _, err := NewPostgresPools(lc, cfg, log, resources.NameUserDB, testAuditDB); err != nil {
		t.Fatalf("NewPostgresPools: %v", err)
	}
	lc.RequireStart()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := lc.Stop(ctx)
	if err == nil {
		t.Fatal("Lifecycle Stop error = nil")
	}
	if !strings.Contains(err.Error(), "close postgres user_db") {
		t.Fatalf("Lifecycle Stop error = %q, want user_db context", err.Error())
	}
	if !strings.Contains(err.Error(), "close postgres audit_db") {
		t.Fatalf("Lifecycle Stop error = %q, want audit_db context", err.Error())
	}
	if got := drv.closes.Load(); got != 2 {
		t.Fatalf("closes = %d, want 2", got)
	}
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
	if err == nil {
		t.Fatal("ValidateApp error = nil")
	}
	if !strings.Contains(err.Error(), `name="user_db"`) {
		t.Fatalf("ValidateApp error = %q, want missing named user_db", err.Error())
	}
}

func TestNewRedisClientReturnsErrorForMissingConfig(t *testing.T) {
	cfg := &config.Config{}
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := NewRedisClient(lc, cfg, log, "missing_redis")
	if err == nil {
		t.Fatal("NewRedisClient error = nil")
	}
	if !strings.Contains(err.Error(), `redis config "missing_redis" not found`) {
		t.Fatalf("NewRedisClient error = %q, want missing config", err.Error())
	}
}

func TestNewRedisClientRegistersLifecycle(t *testing.T) {
	redisServer := newTestRedisServer(t)
	cfg := &config.Config{Redis: map[string]config.RedisConfig{
		resources.NameCacheRedis: {
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

	client, err := NewRedisClient(lc, cfg, log, resources.NameCacheRedis)
	if err != nil {
		t.Fatalf("NewRedisClient: %v", err)
	}
	lc.RequireStart()
	lc.RequireStop()

	if client.Options().Addr != redisServer.addr {
		t.Fatalf("Addr = %q, want %q", client.Options().Addr, redisServer.addr)
	}
	if got := redisServer.pings.Load(); got != 1 {
		t.Fatalf("pings = %d, want 1", got)
	}
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
	if err == nil {
		t.Fatal("ValidateApp error = nil")
	}
	if !strings.Contains(err.Error(), `name="cache_redis"`) {
		t.Fatalf("ValidateApp error = %q, want missing named cache_redis", err.Error())
	}
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
		ProvideNamedPostgres(resources.NameUserDB, resources.NameUserDB),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	if got.UserDB == nil {
		t.Fatal("UserDB = nil")
	}
	if got := drv.pings.Load(); got != 1 {
		t.Fatalf("pings = %d, want 1", got)
	}
}

func TestProvideNamedRedisProvidesOnlyDeclaredClient(t *testing.T) {
	redisServer := newTestRedisServer(t)
	cfg := &config.Config{Redis: map[string]config.RedisConfig{
		resources.NameCacheRedis: {
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
		ProvideNamedRedis(resources.NameCacheRedis, resources.NameCacheRedis),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	if got.CacheRedis == nil {
		t.Fatal("CacheRedis = nil")
	}
	if got := redisServer.pings.Load(); got != 1 {
		t.Fatalf("pings = %d, want 1", got)
	}
}

func testConfig(driverName string) *config.Config {
	return &config.Config{
		Postgres: map[string]config.PostgresConfig{
			resources.NameUserDB: {
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
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
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
		command := strings.ToUpper(string(buf[:n]))
		switch {
		case strings.Contains(command, "PING"):
			s.pings.Add(1)
			_, _ = conn.Write([]byte("+PONG\r\n"))
		case strings.Contains(command, "HELLO"):
			_, _ = conn.Write([]byte("-ERR unknown command 'HELLO'\r\n"))
		case strings.Contains(command, "CLIENT"):
			_, _ = conn.Write([]byte("+OK\r\n"))
		default:
			_, _ = conn.Write([]byte("+OK\r\n"))
		}
	}
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
	return nil
}
