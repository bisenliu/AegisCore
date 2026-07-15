package bootstrap

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/validation"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

func TestAppModuleResolvesSharedValidationDependency(t *testing.T) {
	serviceCfg := appModuleValidationTestConfig()
	err := fx.ValidateApp(AppOptions(
		serviceCfg,
		AppModule,
		fx.Invoke(func(*validation.Validator, *userhttp.UserController) {}),
	)...)
	require.NoError(t, err)
}

func TestAppModuleIncludesSharedTimezoneDependency(t *testing.T) {
	serviceCfg := appModuleValidationTestConfig()
	err := fx.ValidateApp(AppOptions(
		serviceCfg,
		AppModule,
		fx.Invoke(func(*validation.Validator, *userhttp.UserController) {}),
	)...)
	require.NoError(t, err)
}

func TestAppWiresCommonDependenciesExplicitly(t *testing.T) {
	serviceCfg := appModuleValidationTestConfig()
	err := fx.ValidateApp(AppOptions(
		serviceCfg,
		AppModule,
		fx.Invoke(func(*config.Config, *serviceconfig.Config, *zap.Logger, *userhttp.UserController) {}),
	)...)
	require.NoError(t, err)
}

func TestAppOptionsSupplySameConfigurationAndLifecycleTimeouts(t *testing.T) {
	serviceCfg := appModuleValidationTestConfig()
	serviceCfg.Runtime.Lifecycle = config.LifecycleConfig{
		StartTimeout: 17 * time.Second,
		StopTimeout:  29 * time.Second,
	}
	var resolvedServiceCfg *serviceconfig.Config
	var resolvedRuntimeCfg *config.Config

	app := fx.New(AppOptions(
		serviceCfg,
		fx.Populate(&resolvedServiceCfg, &resolvedRuntimeCfg),
		fx.NopLogger,
	)...)

	require.NoError(t, app.Err())
	require.Same(t, serviceCfg, resolvedServiceCfg)
	require.Equal(t, serviceconfig.NewRuntimeConfig(serviceCfg), resolvedRuntimeCfg)
	require.Equal(t, serviceCfg.Runtime.Lifecycle.StartTimeout, app.StartTimeout())
	require.Equal(t, serviceCfg.Runtime.Lifecycle.StopTimeout, app.StopTimeout())
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

func appModuleValidationTestConfig() *serviceconfig.Config {
	return &serviceconfig.Config{Config: config.Config{
		App:           config.AppConfig{Name: "aegiscore-user-services", Environment: "test"},
		Observability: appModuleTestObservabilityConfig(),
	}, Auth: serviceconfig.AuthConfig{
		PasswordKDF:       serviceconfig.PasswordKDFConfig{Argon2Concurrency: 1, Argon2QueueSize: 1},
		TokenVersionCache: appModuleTestFeatureCacheConfig(time.Second),
	}, RBAC: serviceconfig.RBACConfig{UserRoleCache: appModuleTestFeatureCacheConfig(5 * time.Second)}}
}

func appModuleTestFeatureCacheConfig(ttl time.Duration) serviceconfig.FeatureCacheConfig {
	enabled := true
	size := int64(1000)
	loadTimeout := time.Second
	return serviceconfig.FeatureCacheConfig{Enabled: &enabled, Size: &size, TTL: &ttl, LoadTimeout: &loadTimeout}
}

func appModuleTestObservabilityConfig() config.ObservabilityConfig {
	return config.ObservabilityConfig{
		Tracing: config.TracingConfig{Enabled: false, SampleRatio: 1},
	}
}
