package providers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/user-service/ent"
)

var providerTestDriverSeq atomic.Int64

func TestProvidePostgresPoolsProvidesUserDatabase(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	cfg := providerTestConfig(drv.name)
	log := zap.NewNop()

	type pools struct {
		fx.In

		UserDB *sql.DB `name:"user_db"`
	}

	var got pools
	app := fxtest.New(t,
		fx.Supply(cfg, log),
		fx.Provide(ProvidePostgresPools),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, got.UserDB)

	dbNames := drv.databaseNames()
	want := []string{"aegiscore_user"}
	require.ElementsMatch(t, want, dbNames)
	require.Equal(t, int64(1), drv.pings.Load())
	require.Equal(t, int64(1), drv.closes.Load())
}

func TestProvidePostgresPoolsDoesNotRequireSharedDBConfig(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	cfg := providerTestConfig(drv.name)
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	got, err := ProvidePostgresPools(NamedPostgresParams{
		Lifecycle: lc,
		Config:    cfg,
		Log:       log,
	})
	require.NoError(t, err)
	require.NotNil(t, got.UserDB)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, lc.Start(ctx))
	require.NoError(t, lc.Stop(ctx))
	require.Equal(t, int64(1), drv.pings.Load())
	require.Equal(t, int64(1), drv.closes.Load())
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

func TestPostgresPoolsAndEntClientsClosePoolOnce(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	cfg := providerTestConfig(drv.name)
	log := zap.NewNop()

	type clients struct {
		fx.In

		UserClient *ent.Client `name:"user_db"`
	}

	var got clients
	app := fxtest.New(t,
		fx.Supply(cfg, log),
		fx.Provide(ProvidePostgresPools, ProvideEntClients),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, got.UserClient)
	require.Equal(t, int64(1), drv.pings.Load())
	require.Equal(t, int64(1), drv.closes.Load())
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

	require.NotNil(t, got.CacheRedis)
	require.Equal(t, redisServer.addr, got.CacheRedis.Options().Addr)
	require.Equal(t, int64(1), redisServer.pings.Load())
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
	require.ErrorContains(t, err, `name="queue_redis"`)
}

func TestProvideRedisClientsReturnsErrorForMissingCacheRedisConfig(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	log := zap.NewNop()

	_, err := ProvideRedisClients(NamedRedisParams{
		Lifecycle: lc,
		Config:    &config.Config{},
		Log:       log,
	})
	require.ErrorContains(t, err, `redis config "`+resources.NameCacheRedis+`" not found`)
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

	_, err := ProvideRedisClients(NamedRedisParams{Lifecycle: lc, Config: cfg, Log: log})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Start(ctx)
	require.ErrorContains(t, err, "ping redis cache_redis")
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
