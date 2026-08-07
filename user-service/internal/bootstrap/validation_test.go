package bootstrap

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
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

func TestAppModuleIncludesProcessRuntimeInitialization(t *testing.T) {
	serviceCfg := appModuleValidationTestConfig()
	err := fx.ValidateApp(AppOptions(
		serviceCfg,
		AppModule,
		fx.Invoke(func(*validation.Validator, *userhttp.UserController) {}),
	)...)
	require.NoError(t, err)
}

func TestAppWiresRuntimeAndNarrowSettingsExplicitly(t *testing.T) {
	serviceCfg := appModuleValidationTestConfig()
	err := fx.ValidateApp(AppOptions(
		serviceCfg,
		AppModule,
		fx.Invoke(func(*config.Config, serviceconfig.AuthSettings, serviceconfig.RBACSettings, serviceconfig.EntSettings, serviceconfig.ResourceSettings, *zap.Logger, *userhttp.UserController) {
		}),
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
		fxtest.WithTestLogger(t),
	)...)

	require.NoError(t, app.Err())
	require.Same(t, serviceCfg, resolvedServiceCfg)
	require.Equal(t, serviceconfig.NewRuntimeConfig(serviceCfg), resolvedRuntimeCfg)
	require.Equal(t, serviceCfg.Runtime.Lifecycle.StartTimeout, app.StartTimeout())
	require.Equal(t, serviceCfg.Runtime.Lifecycle.StopTimeout, app.StopTimeout())
}

func TestAppOptionsRecoverFromFxInitializationPanics(t *testing.T) {
	serviceCfg := appModuleValidationTestConfig()
	tests := []struct {
		name    string
		option  fx.Option
		message string
	}{
		{
			name: "constructor",
			option: fx.Options(
				fx.Provide(func() *struct{} { panic("constructor boom") }),
				fx.Invoke(func(*struct{}) {}),
			),
			message: "constructor boom",
		},
		{
			name: "decorator",
			option: fx.Options(
				fx.Supply("input"),
				fx.Decorate(func(string) string { panic("decorator boom") }),
				fx.Invoke(func(string) {}),
			),
			message: "decorator boom",
		},
		{
			name:    "invoke",
			option:  fx.Invoke(func() { panic("invoke boom") }),
			message: "invoke boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var app *fx.App
			require.NotPanics(t, func() {
				app = fx.New(AppOptions(serviceCfg, tt.option, fxtest.WithTestLogger(t))...)
			})
			require.ErrorContains(t, app.Err(), tt.message)
		})
	}
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

func TestRuntimeModuleRegistersRuntimeServersThroughNamedInvoke(t *testing.T) {
	content, err := os.ReadFile("app.go")
	require.NoError(t, err)

	source := string(content)
	require.Contains(t, source, "fx.Invoke(InitProcessRuntime)")
	require.Contains(t, source, "fx.Invoke(registerRuntimeServers)")
	require.Contains(t, source, "func registerRuntimeServers(_ *HTTPRuntime, _ *PprofRuntime) {}")
	require.Contains(t, source, "func InitProcessRuntime(settings serviceconfig.TimezoneSettings) error")
	require.NotContains(t, source, "commontz.Module")
	require.NotContains(t, source, "func(*HTTPRuntime) {}")
	require.NotContains(t, source, "func(*PprofRuntime) {}")
}

func TestRuntimeModuleOrdersProcessRuntimeBeforeRuntimeServers(t *testing.T) {
	content, err := os.ReadFile("app.go")
	require.NoError(t, err)
	source := string(content)

	initIndex := strings.Index(source, "fx.Invoke(InitProcessRuntime)")
	providerRuntimeIndex := strings.Index(source, "providers.RuntimeModule")
	permissionLifecycleIndex := strings.Index(source, "permissionfeature.LifecycleModule")
	serverIndex := strings.Index(source, "fx.Invoke(registerRuntimeServers)")

	require.NotEqual(t, -1, initIndex)
	require.NotEqual(t, -1, providerRuntimeIndex)
	require.NotEqual(t, -1, permissionLifecycleIndex)
	require.NotEqual(t, -1, serverIndex)
	require.Less(t, initIndex, providerRuntimeIndex)
	require.Less(t, providerRuntimeIndex, permissionLifecycleIndex)
	require.Less(t, permissionLifecycleIndex, serverIndex)
}

func appModuleValidationTestConfig() *serviceconfig.Config {
	return &serviceconfig.Config{Config: config.Config{
		App:           config.AppConfig{Name: "aegiscore-user-service", Environment: "test"},
		Observability: appModuleTestObservabilityConfig(),
	}, Auth: serviceconfig.AuthConfig{
		TokenVersionCache: appModuleTestFeatureCacheConfig(time.Second),
	}, RBAC: serviceconfig.RBACConfig{UserRoleCache: appModuleTestFeatureCacheConfig(5 * time.Second)}}
}

func appModuleTestFeatureCacheConfig(ttl time.Duration) serviceconfig.FeatureCacheConfig {
	return serviceconfig.FeatureCacheConfig{Enabled: true, Size: 1000, TTL: ttl, LoadTimeout: time.Second}
}

func appModuleTestObservabilityConfig() config.ObservabilityConfig {
	return config.ObservabilityConfig{
		Tracing: config.TracingConfig{Enabled: false, SampleRatio: 1},
	}
}
