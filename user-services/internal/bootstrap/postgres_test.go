package bootstrap

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	commoninfra "github.com/aegiscore/common/infrastructure"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

var bootstrapTestDriverSeq atomic.Int64

func TestNewPostgresPoolsProvidesUserServiceDatabases(t *testing.T) {
	drv := registerBootstrapTestSQLDriver(t)
	cfg := bootstrapTestConfig(drv.name)
	log := zap.NewNop()

	type pools struct {
		fx.In

		UserDB   *sql.DB `name:"user_db"`
		CommonDB *sql.DB `name:"common_db"`
	}

	var got pools
	app := fxtest.New(t,
		fx.Supply(cfg, log),
		fx.Provide(NewPostgresPools),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	if got.UserDB == nil {
		t.Fatal("UserDB = nil")
	}
	if got.CommonDB == nil {
		t.Fatal("CommonDB = nil")
	}

	dbNames := drv.databaseNames()
	want := []string{"aegiscore_common", "aegiscore_user"}
	if strings.Join(dbNames, ",") != strings.Join(want, ",") {
		t.Fatalf("opened databases = %v, want %v", dbNames, want)
	}
	if got := drv.pings.Load(); got != 2 {
		t.Fatalf("pings = %d, want 2", got)
	}
	if got := drv.closes.Load(); got != 2 {
		t.Fatalf("closes = %d, want 2", got)
	}
}

func TestNewPostgresPoolsDoesNotProvidePayDatabase(t *testing.T) {
	type pools struct {
		fx.In

		PayDB *sql.DB `name:"pay_db"`
	}

	err := fx.ValidateApp(
		fx.Provide(NewPostgresPools),
		fx.Invoke(func(pools) {}),
	)
	if err == nil {
		t.Fatal("ValidateApp error = nil")
	}
	if !strings.Contains(err.Error(), `name="pay_db"`) {
		t.Fatalf("ValidateApp error = %q, want missing named pay_db", err.Error())
	}
}

func TestNewRedisClientsProvidesCacheRedis(t *testing.T) {
	redisServer := newBootstrapTestRedisServer(t)
	cfg := bootstrapTestConfig("")
	cfg.Redis = map[string]config.RedisConfig{
		commoninfra.NameCacheRedis: {
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
	}
	log := zap.NewNop()

	type clients struct {
		fx.In

		CacheRedis *redis.Client `name:"cache_redis"`
	}

	var got clients
	app := fxtest.New(t,
		fx.Supply(cfg, log),
		fx.Provide(NewRedisClients),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	if got.CacheRedis == nil {
		t.Fatal("CacheRedis = nil")
	}
	if got.CacheRedis.Options().Addr != redisServer.addr {
		t.Fatalf("CacheRedis addr = %q, want %q", got.CacheRedis.Options().Addr, redisServer.addr)
	}
	if got := redisServer.pings.Load(); got != 1 {
		t.Fatalf("redis pings = %d, want 1", got)
	}
	redisServer.requireClosed(t)
}

func TestNewRedisClientsDoesNotProvideQueueRedis(t *testing.T) {
	type clients struct {
		fx.In

		QueueRedis *redis.Client `name:"queue_redis"`
	}

	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		fx.Provide(NewRedisClients),
		fx.Invoke(func(clients) {}),
	)
	if err == nil {
		t.Fatal("ValidateApp error = nil")
	}
	if !strings.Contains(err.Error(), `name="queue_redis"`) {
		t.Fatalf("ValidateApp error = %q, want missing named queue_redis", err.Error())
	}
}

func TestNewRedisClientsReturnsErrorForMissingCacheRedisConfig(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := NewRedisClients(NamedRedisParams{
		Lifecycle: lc,
		Config:    &config.Config{},
		Log:       log,
	})
	if err == nil {
		t.Fatal("NewRedisClients error = nil")
	}
	if !strings.Contains(err.Error(), `redis config "`+commoninfra.NameCacheRedis+`" not found`) {
		t.Fatalf("NewRedisClients error = %q, want missing cache_redis config", err.Error())
	}
}

func TestNewRedisClientsFailsStartWhenCacheRedisUnavailable(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()
	cfg := &config.Config{Redis: map[string]config.RedisConfig{
		commoninfra.NameCacheRedis: {
			Addr:         "127.0.0.1:1",
			DB:           0,
			DialTimeout:  10 * time.Millisecond,
			ReadTimeout:  10 * time.Millisecond,
			WriteTimeout: 10 * time.Millisecond,
			PingTimeout:  10 * time.Millisecond,
		},
	}}

	if _, err := NewRedisClients(NamedRedisParams{Lifecycle: lc, Config: cfg, Log: log}); err != nil {
		t.Fatalf("NewRedisClients: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := lc.Start(ctx); err == nil {
		t.Fatal("Lifecycle Start error = nil")
	} else if !strings.Contains(err.Error(), "ping redis cache_redis") {
		t.Fatalf("Lifecycle Start error = %q, want ping redis cache_redis", err.Error())
	}
}

func bootstrapTestConfig(driverName string) *config.Config {
	return &config.Config{
		Redis: map[string]config.RedisConfig{
			commoninfra.NameCacheRedis: {
				Addr:         "127.0.0.1:6379",
				DB:           0,
				DialTimeout:  time.Second,
				ReadTimeout:  time.Second,
				WriteTimeout: time.Second,
				PingTimeout:  time.Second,
			},
		},
		PostgresConfigs: map[string]config.PostgresConfig{
			commoninfra.NameUserDB: {
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
			"pay_db": {
				Host:            "127.0.0.1",
				Port:            15432,
				Username:        "aegiscore",
				Password:        "secret",
				DBName:          "aegiscore_pay",
				Driver:          driverName,
				MaxOpenConns:    7,
				MaxIdleConns:    3,
				ConnMaxLifetime: time.Minute,
				ConnMaxIdleTime: 30 * time.Second,
				PingTimeout:     time.Second,
			},
			commoninfra.NameCommonDB: {
				Host:            "127.0.0.1",
				Port:            15432,
				Username:        "aegiscore",
				Password:        "secret",
				DBName:          "aegiscore_common",
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

func newBootstrapTestRedisServer(t *testing.T) *bootstrapTestRedisServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	server := &bootstrapTestRedisServer{
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

type bootstrapTestRedisServer struct {
	addr   string
	pings  atomic.Int64
	closed chan struct{}
}

func (s *bootstrapTestRedisServer) handle(conn net.Conn) {
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

func (s *bootstrapTestRedisServer) requireClosed(t *testing.T) {
	t.Helper()
	select {
	case <-s.closed:
	case <-time.After(time.Second):
		t.Fatal("redis connection was not closed")
	}
}

func registerBootstrapTestSQLDriver(t *testing.T) *bootstrapTestSQLDriver {
	t.Helper()
	drv := &bootstrapTestSQLDriver{name: fmt.Sprintf("aegiscore_bootstrap_test_postgres_%d", bootstrapTestDriverSeq.Add(1))}
	sql.Register(drv.name, drv)
	return drv
}

type bootstrapTestSQLDriver struct {
	name   string
	pings  atomic.Int64
	closes atomic.Int64

	mu   sync.Mutex
	dsns []string
}

func (d *bootstrapTestSQLDriver) Open(dsn string) (driver.Conn, error) {
	d.mu.Lock()
	d.dsns = append(d.dsns, dsn)
	d.mu.Unlock()
	return &bootstrapTestSQLConn{driver: d}, nil
}

func (d *bootstrapTestSQLDriver) databaseNames() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbNames := make([]string, 0, len(d.dsns))
	for _, dsn := range d.dsns {
		parsed, err := url.Parse(dsn)
		if err != nil {
			continue
		}
		dbNames = append(dbNames, strings.TrimPrefix(parsed.Path, "/"))
	}
	sort.Strings(dbNames)
	return dbNames
}

type bootstrapTestSQLConn struct {
	driver *bootstrapTestSQLDriver
}

func (c *bootstrapTestSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}

func (c *bootstrapTestSQLConn) Close() error {
	c.driver.closes.Add(1)
	return nil
}

func (c *bootstrapTestSQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not implemented")
}

func (c *bootstrapTestSQLConn) Ping(context.Context) error {
	c.driver.pings.Add(1)
	return nil
}
