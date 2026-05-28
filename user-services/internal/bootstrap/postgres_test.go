package bootstrap

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

var bootstrapTestDriverSeq atomic.Int64

func TestNewPostgresPoolsProvidesUserServiceDatabases(t *testing.T) {
	drv := registerBootstrapTestSQLDriver(t)
	cfg := bootstrapTestConfig(drv.name)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

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

func bootstrapTestConfig(driverName string) *config.Config {
	return &config.Config{
		Database: config.DatabaseConfig{
			Postgres: config.PostgresConfig{
				Host:            "127.0.0.1",
				Port:            15432,
				Username:        "aegiscore",
				Password:        "secret",
				UserDBName:      "aegiscore_user",
				PayDBName:       "aegiscore_pay",
				CommonDBName:    "aegiscore_common",
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
