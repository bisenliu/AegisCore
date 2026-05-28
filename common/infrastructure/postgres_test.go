package infrastructure

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

var testDriverSeq atomic.Int64

func TestNewPostgresReturnsErrorForMissingConfig(t *testing.T) {
	cfg := &config.Config{}
	lc := fxtest.NewLifecycle(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

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
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

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
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

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

func testConfig(driverName string) *config.Config {
	return &config.Config{
		Database: config.DatabaseConfig{
			Postgres: config.PostgresConfig{
				Host:            "127.0.0.1",
				Port:            15432,
				Username:        "aegiscore",
				Password:        "secret",
				UserDBName:      "aegiscore_user",
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
