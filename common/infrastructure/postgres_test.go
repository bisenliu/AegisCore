package infrastructure

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

	"github.com/aegiscore/common/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

var testDriverSeq atomic.Int64

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

	db, err := NewPostgres(lc, cfg, log, "user_db")
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

	if _, err := NewPostgres(lc, cfg, log, "user_db"); err != nil {
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

func TestModuleDoesNotProvideNamedPostgresPools(t *testing.T) {
	type params struct {
		fx.In

		UserDB *sql.DB `name:"user_db"`
	}

	err := fx.ValidateApp(
		Module,
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
		"cache_redis": {
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

	client, err := NewRedisClient(lc, cfg, log, "cache_redis")
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

func TestModuleDoesNotProvideRedisClient(t *testing.T) {
	type params struct {
		fx.In

		Redis any `name:"cache_redis"`
	}

	err := fx.ValidateApp(
		Module,
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
		ProvideNamedPostgres("user_db", "user_db"),
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
		"cache_redis": {
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
		ProvideNamedRedis("cache_redis", "cache_redis"),
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
		PostgresConfigs: map[string]config.PostgresConfig{
			"user_db": {
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
	return nil
}

func (c *testSQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not implemented")
}

func (c *testSQLConn) Ping(context.Context) error {
	c.driver.pings.Add(1)
	return nil
}
