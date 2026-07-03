package bootstrap

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/common/validation"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

var appModuleTestDriverSeq atomic.Int64

func TestAppModuleResolvesSharedValidationDependency(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(appModuleValidationTestConfig(), zap.NewNop()),
		AppModule,
		fx.Invoke(func(*validation.Validator, *userhttp.UserController) {}),
	)
	require.NoError(t, err)
}

func TestAppModuleIncludesSharedTimezoneDependency(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(appModuleValidationTestConfig(), zap.NewNop()),
		AppModule,
		fx.Invoke(func(*validation.Validator, *userhttp.UserController) {}),
	)
	require.NoError(t, err)
}

func TestAppWiresCommonDependenciesExplicitly(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(config.ConfigPath("../../configs/config.yaml")),
		fx.Provide(
			config.NewConfig,
			logger.NewLogger,
		),
		AppModule,
		fx.Invoke(func(*config.Config, *zap.Logger, *userhttp.UserController) {}),
	)
	require.NoError(t, err)
}

func TestRuntimeModuleNamingReflectsCompositionRootScope(t *testing.T) {
	content, err := os.ReadFile("app.go")
	require.NoError(t, err)

	source := string(content)
	legacyName := "User" + "ServiceModule"
	require.NotContains(t, source, legacyName)
	supersededName := "User" + "ServiceRuntimeModule"
	require.NotContains(t, source, supersededName)
	require.Contains(t, source, "AppModule")
}

func TestAppModuleStopsHTTPBeforeDatastoreResources(t *testing.T) {
	drv := registerAppModuleTestSQLDriver(t)
	redisServer := miniredis.RunT(t)
	httpPort := reserveHTTPTestPort(t)
	core, logs := observer.New(zapcore.InfoLevel)
	cfg := appModuleLifecycleTestConfig(drv.name, redisServer.Addr(), httpPort)

	app := fxtest.New(t,
		fx.Supply(cfg, zap.New(core)),
		AppModule,
	)
	app.RequireStart()
	app.RequireStop()

	messages := make([]string, 0, logs.Len())
	for _, entry := range logs.All() {
		messages = append(messages, entry.Message)
	}
	httpStopIndex := indexMessage(messages, "stopping http server")
	redisCloseIndex := indexMessage(messages, "redis closed")
	postgresCloseIndex := indexMessage(messages, "postgres closed")
	require.GreaterOrEqual(t, httpStopIndex, 0, "messages=%v", messages)
	require.GreaterOrEqual(t, redisCloseIndex, 0, "messages=%v", messages)
	require.GreaterOrEqual(t, postgresCloseIndex, 0, "messages=%v", messages)
	require.Less(t, httpStopIndex, redisCloseIndex, "messages=%v", messages)
	require.Less(t, httpStopIndex, postgresCloseIndex, "messages=%v", messages)
	require.Equal(t, int64(1), drv.closes.Load())
}

func appModuleLifecycleTestConfig(driverName string, redisAddr string, httpPort int) *config.Config {
	return &config.Config{
		System: config.SystemConfig{Timezone: "Asia/Shanghai"},
		App:    config.AppConfig{Name: "aegiscore-user-services", Environment: "test"},
		HTTP: config.HTTPConfig{
			Host:            "127.0.0.1",
			Port:            httpPort,
			ReadTimeout:     time.Second,
			WriteTimeout:    time.Second,
			IdleTimeout:     time.Second,
			ShutdownTimeout: time.Second,
		},
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				Secret:          "secret",
				Issuer:          "aegiscore-user-services",
				Audience:        "aegiscore-users",
				AccessTokenTTL:  time.Minute,
				RefreshTokenTTL: time.Hour,
			},
			PasswordKDF:              config.PasswordKDFConfig{Argon2Concurrency: 1, Argon2QueueSize: 1},
			TokenVersionCacheTTL:     time.Minute,
			MaxActiveSessionsPerUser: 5,
		},
		LocalCache: appModuleTestLocalCacheConfig(),
		Redis: map[string]config.RedisConfig{
			resources.NameCacheRedis: {
				Addr:         redisAddr,
				DialTimeout:  time.Second,
				ReadTimeout:  time.Second,
				WriteTimeout: time.Second,
				PingTimeout:  time.Second,
			},
		},
		Postgres: map[string]config.PostgresConfig{
			resources.NameUserDB: appModulePostgresConfig(driverName, "aegiscore_user"),
		},
		Observability: appModuleTestObservabilityConfig(),
	}
}

func appModuleValidationTestConfig() *config.Config {
	return &config.Config{
		App:           config.AppConfig{Name: "aegiscore-user-services", Environment: "test"},
		LocalCache:    appModuleTestLocalCacheConfig(),
		Observability: appModuleTestObservabilityConfig(),
	}
}

func appModuleTestLocalCacheConfig() config.LocalCacheConfig {
	return config.LocalCacheConfig{
		"auth_token_version": config.LocalCacheInstanceConfig{Capacity: 1000, TTL: time.Second, LoadTimeout: time.Second},
		"rbac_user_roles":    config.LocalCacheInstanceConfig{Capacity: 1000, TTL: time.Second, LoadTimeout: time.Second},
	}
}

func appModuleTestObservabilityConfig() config.ObservabilityConfig {
	return config.ObservabilityConfig{
		Tracing: config.TracingConfig{Enabled: true, SampleRatio: 1, Exporter: "none"},
	}
}

func appModulePostgresConfig(driverName string, dbName string) config.PostgresConfig {
	return config.PostgresConfig{
		Host:            "127.0.0.1",
		Port:            15432,
		Username:        "aegiscore",
		Password:        "secret",
		DBName:          dbName,
		Driver:          driverName,
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Minute,
		PingTimeout:     time.Second,
	}
}

func indexMessage(messages []string, want string) int {
	for i, message := range messages {
		if message == want {
			return i
		}
	}
	return -1
}

func registerAppModuleTestSQLDriver(t *testing.T) *appModuleTestSQLDriver {
	t.Helper()

	drv := &appModuleTestSQLDriver{name: fmt.Sprintf("aegiscore_app_module_test_postgres_%d", appModuleTestDriverSeq.Add(1))}
	sql.Register(drv.name, drv)
	return drv
}

type appModuleTestSQLDriver struct {
	name   string
	closes atomic.Int64
}

func (d *appModuleTestSQLDriver) Open(_ string) (driver.Conn, error) {
	return &appModuleTestSQLConn{driver: d}, nil
}

type appModuleTestSQLConn struct {
	driver *appModuleTestSQLDriver
}

func (c *appModuleTestSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}

func (c *appModuleTestSQLConn) Close() error {
	c.driver.closes.Add(1)
	return nil
}

func (c *appModuleTestSQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not implemented")
}

func (c *appModuleTestSQLConn) Ping(context.Context) error {
	return nil
}
