package providers

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

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/user-service/ent"
)

var providerTestDriverSeq atomic.Int64

func TestProvidePostgresPoolsProvidesUserServiceDatabases(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	cfg := providerTestConfig(drv.name)
	log := zap.NewNop()

	type pools struct {
		fx.In

		UserDB   *sql.DB `name:"user_db"`
		CommonDB *sql.DB `name:"common_db"`
	}

	var got pools
	app := fxtest.New(t,
		fx.Supply(cfg, log),
		fx.Provide(ProvidePostgresPools),
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

func TestProvidePostgresPoolsReturnsErrorForMissingCommonDBConfig(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	cfg := providerTestConfig(drv.name)
	delete(cfg.Postgres, resources.NameCommonDB)
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := ProvidePostgresPools(NamedPostgresParams{
		Lifecycle: lc,
		Config:    cfg,
		Log:       log,
	})
	if err == nil {
		t.Fatal("ProvidePostgresPools error = nil")
	}
	if !strings.Contains(err.Error(), `postgres config "`+resources.NameCommonDB+`" not found`) {
		t.Fatalf("ProvidePostgresPools error = %q, want common_db context", err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := lc.Start(ctx); err != nil {
		t.Fatalf("lifecycle start after failed provider: %v", err)
	}
	if err := lc.Stop(ctx); err != nil {
		t.Fatalf("lifecycle stop after failed provider: %v", err)
	}
	if got := drv.pings.Load(); got != 0 {
		t.Fatalf("pings after failed provider = %d, want 0", got)
	}
	if got := drv.closes.Load(); got != 0 {
		t.Fatalf("driver closes after failed provider = %d, want 0", got)
	}
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
	if err == nil {
		t.Fatal("ValidateApp error = nil")
	}
	if !strings.Contains(err.Error(), `name="pay_db"`) {
		t.Fatalf("ValidateApp error = %q, want missing named pay_db", err.Error())
	}
}

func TestProvideEntClientsProvidesUserServiceEntClients(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	userDB, err := sql.Open(drv.name, "postgres://aegiscore:secret@127.0.0.1/aegiscore_user")
	if err != nil {
		t.Fatalf("open user db: %v", err)
	}
	commonDB, err := sql.Open(drv.name, "postgres://aegiscore:secret@127.0.0.1/aegiscore_common")
	if err != nil {
		t.Fatalf("open common db: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := userDB.PingContext(ctx); err != nil {
		t.Fatalf("ping user db: %v", err)
	}
	if err := commonDB.PingContext(ctx); err != nil {
		t.Fatalf("ping common db: %v", err)
	}

	lc := fxtest.NewLifecycle(t)
	got := ProvideEntClients(NamedEntClientParams{
		Lifecycle: lc,
		Log:       zap.NewNop(),
		UserDB:    userDB,
		CommonDB:  commonDB,
	})

	if got.UserClient == nil {
		t.Fatal("UserClient = nil")
	}
	if got.CommonClient == nil {
		t.Fatal("CommonClient = nil")
	}
	if got := drv.closes.Load(); got != 0 {
		t.Fatalf("closes before lifecycle stop = %d, want 0", got)
	}

	if err := lc.Start(ctx); err != nil {
		t.Fatalf("lifecycle start: %v", err)
	}
	if err := lc.Stop(ctx); err != nil {
		t.Fatalf("lifecycle stop: %v", err)
	}
	if got := drv.closes.Load(); got != 0 {
		t.Fatalf("closes after ent lifecycle stop = %d, want 0", got)
	}
}

func TestPostgresPoolsAndEntClientsClosePoolsOnce(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	cfg := providerTestConfig(drv.name)
	log := zap.NewNop()

	type clients struct {
		fx.In

		UserClient   *ent.Client `name:"user_db"`
		CommonClient *ent.Client `name:"common_db"`
	}

	var got clients
	app := fxtest.New(t,
		fx.Supply(cfg, log),
		fx.Provide(ProvidePostgresPools, ProvideEntClients),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	if got.UserClient == nil {
		t.Fatal("UserClient = nil")
	}
	if got.CommonClient == nil {
		t.Fatal("CommonClient = nil")
	}
	if got := drv.pings.Load(); got != 2 {
		t.Fatalf("pings = %d, want 2", got)
	}
	if got := drv.closes.Load(); got != 2 {
		t.Fatalf("closes after composed lifecycle stop = %d, want 2", got)
	}
}

func TestProvideRedisClientsProvidesCacheRedis(t *testing.T) {
	redisServer := newProviderTestRedisServer(t)
	cfg := providerTestConfig("")
	cfg.Redis = map[string]config.RedisConfig{
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
	}
	log := zap.NewNop()

	type clients struct {
		fx.In

		CacheRedis *redis.Client `name:"cache_redis"`
	}

	var got clients
	app := fxtest.New(t,
		fx.Supply(cfg, log),
		fx.Provide(ProvideRedisClients),
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

func TestProvideRedisClientsDoesNotProvideQueueRedis(t *testing.T) {
	type clients struct {
		fx.In

		QueueRedis *redis.Client `name:"queue_redis"`
	}

	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		fx.Provide(ProvideRedisClients),
		fx.Invoke(func(clients) {}),
	)
	if err == nil {
		t.Fatal("ValidateApp error = nil")
	}
	if !strings.Contains(err.Error(), `name="queue_redis"`) {
		t.Fatalf("ValidateApp error = %q, want missing named queue_redis", err.Error())
	}
}

func TestProvideRedisClientsReturnsErrorForMissingCacheRedisConfig(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := ProvideRedisClients(NamedRedisParams{
		Lifecycle: lc,
		Config:    &config.Config{},
		Log:       log,
	})
	if err == nil {
		t.Fatal("ProvideRedisClients error = nil")
	}
	if !strings.Contains(err.Error(), `redis config "`+resources.NameCacheRedis+`" not found`) {
		t.Fatalf("ProvideRedisClients error = %q, want missing cache_redis config", err.Error())
	}
}

func TestProvideRedisClientsFailsStartWhenCacheRedisUnavailable(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()
	cfg := &config.Config{Redis: map[string]config.RedisConfig{
		resources.NameCacheRedis: {
			Addr:         "127.0.0.1:1",
			DB:           0,
			DialTimeout:  10 * time.Millisecond,
			ReadTimeout:  10 * time.Millisecond,
			WriteTimeout: 10 * time.Millisecond,
			PingTimeout:  10 * time.Millisecond,
		},
	}}

	if _, err := ProvideRedisClients(NamedRedisParams{Lifecycle: lc, Config: cfg, Log: log}); err != nil {
		t.Fatalf("ProvideRedisClients: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := lc.Start(ctx); err == nil {
		t.Fatal("Lifecycle Start error = nil")
	} else if !strings.Contains(err.Error(), "ping redis cache_redis") {
		t.Fatalf("Lifecycle Start error = %q, want ping redis cache_redis", err.Error())
	}
}

func providerTestConfig(driverName string) *config.Config {
	return &config.Config{
		Redis: map[string]config.RedisConfig{
			resources.NameCacheRedis: {
				Addr:         "127.0.0.1:6379",
				DB:           0,
				DialTimeout:  time.Second,
				ReadTimeout:  time.Second,
				WriteTimeout: time.Second,
				PingTimeout:  time.Second,
			},
		},
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
			resources.NameCommonDB: {
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

func newProviderTestRedisServer(t *testing.T) *providerTestRedisServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
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

func (s *providerTestRedisServer) requireClosed(t *testing.T) {
	t.Helper()
	select {
	case <-s.closed:
	case <-time.After(time.Second):
		t.Fatal("redis connection was not closed")
	}
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

	mu   sync.Mutex
	dsns []string
}

func (d *providerTestSQLDriver) Open(dsn string) (driver.Conn, error) {
	d.mu.Lock()
	d.dsns = append(d.dsns, dsn)
	d.mu.Unlock()
	return &providerTestSQLConn{driver: d}, nil
}

func (d *providerTestSQLDriver) databaseNames() []string {
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
