package datastore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/resources"
)

var testDriverSeq atomic.Int64

const testPrimaryDB = "primary_db"

func TestOpenPostgresAppliesPoolSettings(t *testing.T) {
	db, err := OpenPostgres(testPrimaryDB, testPostgresConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.Equal(t, 7, db.Stats().MaxOpenConnections)
}

func TestOpenPostgresAppliesDefaultPoolSettings(t *testing.T) {
	db, err := OpenPostgres(testPrimaryDB, resources.PostgresConfig{
		Host: "127.0.0.1", Port: 5432, Username: "aegiscore", DBName: "aegiscore_user",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.Equal(t, resources.DefaultPostgresMaxOpenConns, db.Stats().MaxOpenConnections)
}

func TestPingPostgresUsesDefaultTimeout(t *testing.T) {
	drv := registerTestSQLDriver(t)
	db := openTestPostgres(t, drv)
	require.NoError(t, PingPostgres(context.Background(), testPrimaryDB, db))
	require.Equal(t, int64(1), drv.pings.Load())
	require.Greater(t, time.Duration(drv.pingTimeoutNanos.Load()), 4*time.Second)
	require.LessOrEqual(t, time.Duration(drv.pingTimeoutNanos.Load()), resources.DefaultPostgresPingTimeout())
}

func TestPingPostgresClosesPoolAndJoinsErrors(t *testing.T) {
	drv := registerTestSQLDriver(t)
	drv.pingErr = errors.New("postgres unavailable")
	drv.closeErr = errors.New("driver close failed")
	db := openTestPostgres(t, drv)

	err := PingPostgres(context.Background(), testPrimaryDB, db)
	require.ErrorContains(t, err, "ping postgres primary_db: postgres unavailable")
	require.ErrorContains(t, err, "close postgres primary_db: driver close failed")
	require.Equal(t, int64(1), drv.pings.Load())
	require.Equal(t, int64(1), drv.closes.Load())
}

func TestClosePostgresPreservesResourceName(t *testing.T) {
	drv := registerTestSQLDriver(t)
	drv.closeErr = errors.New("driver close failed")
	db := openTestPostgres(t, drv)
	require.NoError(t, PingPostgres(context.Background(), testPrimaryDB, db))
	require.ErrorContains(t, ClosePostgres(testPrimaryDB, db), "close postgres primary_db: driver close failed")
}

func TestPostgresDSNIncludesEscapedCredentialsDatabaseAndSSLMode(t *testing.T) {
	dsn := PostgresDSN(resources.PostgresConfig{
		Host: "2001:db8::1", Port: 5432, Username: "aegis user", Password: "p@ss:/word", DBName: "user/data", SSLMode: "verify-full",
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

func testPostgresConfig() resources.PostgresConfig {
	return resources.PostgresConfig{
		Host: "127.0.0.1", Port: 15432, Username: "aegiscore", Password: "secret", DBName: "aegiscore_user", SSLMode: "disable",
		Pool: resources.PostgresPoolConfig{MaxOpenConns: 7, MaxIdleConns: 3, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: 30 * time.Second},
	}
}

func openTestPostgres(t *testing.T, drv *testSQLDriver) *sql.DB {
	t.Helper()
	db, err := sql.Open(drv.name, "test")
	require.NoError(t, err)
	return db
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
	pingErr          error
	closeErr         error
}

func (d *testSQLDriver) Open(string) (driver.Conn, error) { return &testSQLConn{driver: d}, nil }

type testSQLConn struct{ driver *testSQLDriver }

func (c *testSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}
func (c *testSQLConn) Begin() (driver.Tx, error) { return nil, errors.New("begin not implemented") }
func (c *testSQLConn) Close() error {
	c.driver.closes.Add(1)
	return c.driver.closeErr
}
func (c *testSQLConn) Ping(ctx context.Context) error {
	c.driver.pings.Add(1)
	if deadline, ok := ctx.Deadline(); ok {
		c.driver.pingTimeoutNanos.Store(int64(time.Until(deadline)))
	}
	return c.driver.pingErr
}
