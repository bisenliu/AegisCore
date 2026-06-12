package bootstrap

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
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
		fx.Supply(&config.Config{}, zap.NewNop()),
		AppModule,
		fx.Invoke(func(*validation.Validator, *userhttp.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp error = %v", err)
	}
}

func TestAppModuleIncludesSharedTimezoneDependency(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		AppModule,
		fx.Invoke(func(*validation.Validator, *userhttp.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp with timezone module error = %v", err)
	}
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
	if err != nil {
		t.Fatalf("ValidateApp with explicit common providers error = %v", err)
	}
}

func TestRuntimeModuleNamingReflectsCompositionRootScope(t *testing.T) {
	content, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatalf("ReadFile app.go error = %v", err)
	}

	source := string(content)
	legacyName := "User" + "ServiceModule"
	if strings.Contains(source, legacyName) {
		t.Fatalf("app.go contains legacy service-layer-like module name %q", legacyName)
	}
	supersededName := "User" + "ServiceRuntimeModule"
	if strings.Contains(source, supersededName) {
		t.Fatalf("app.go contains superseded runtime module name %q", supersededName)
	}
	if !strings.Contains(source, "AppModule") {
		t.Fatal("app.go does not contain composition-root module name AppModule")
	}
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
	if httpStopIndex < 0 {
		t.Fatalf("logs missing stopping http server: %v", messages)
	}
	if redisCloseIndex < 0 {
		t.Fatalf("logs missing redis closed: %v", messages)
	}
	if postgresCloseIndex < 0 {
		t.Fatalf("logs missing postgres closed: %v", messages)
	}
	if httpStopIndex > redisCloseIndex {
		t.Fatalf("log order = %v, want http stop before redis close", messages)
	}
	if httpStopIndex > postgresCloseIndex {
		t.Fatalf("log order = %v, want http stop before postgres close", messages)
	}
	if got := drv.closes.Load(); got != 2 {
		t.Fatalf("postgres closes = %d, want 2", got)
	}
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
			TokenVersionCacheTTL:     time.Minute,
			MaxActiveSessionsPerUser: 5,
		},
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
			resources.NameUserDB:   appModulePostgresConfig(driverName, "aegiscore_user"),
			resources.NameCommonDB: appModulePostgresConfig(driverName, "aegiscore_common"),
		},
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

func (d *appModuleTestSQLDriver) Open(dsn string) (driver.Conn, error) {
	return &appModuleTestSQLConn{driver: d}, nil
}

type appModuleTestSQLConn struct {
	driver *appModuleTestSQLDriver
}

func (c *appModuleTestSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare not implemented")
}

func (c *appModuleTestSQLConn) Close() error {
	c.driver.closes.Add(1)
	return nil
}

func (c *appModuleTestSQLConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("begin not implemented")
}

func (c *appModuleTestSQLConn) Ping(context.Context) error {
	return nil
}
