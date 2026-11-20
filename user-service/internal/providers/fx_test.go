package providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/platform/runtime/config"
	platformmetrics "github.com/aegiscore/platform/runtime/observability/metrics"
	platformtracing "github.com/aegiscore/platform/runtime/observability/tracing"
	"github.com/aegiscore/platform/validation"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authfeature "github.com/aegiscore/user-service/internal/features/auth"
	permissionfeature "github.com/aegiscore/user-service/internal/features/permission"
	rolefeature "github.com/aegiscore/user-service/internal/features/role"
	userfeature "github.com/aegiscore/user-service/internal/features/user"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

func TestModuleResolvesServiceLevelProviders(t *testing.T) {
	serviceCfg := &serviceconfig.Config{
		Config: config.Config{
			App: config.AppConfig{Name: "configured-user-service", Environment: "test"},
			Observability: config.ObservabilityConfig{
				Metrics: config.MetricsConfig{Enabled: false},
				Tracing: config.TracingConfig{Enabled: false, SampleRatio: 1},
			},
		},
		HTTP: serviceconfig.HTTPConfig{RequestBodyMaxBytes: serviceconfig.DefaultHTTPRequestBodyMaxBytes},
	}
	err := fx.ValidateApp(
		fx.Supply(serviceCfg, zap.NewNop()),
		fx.Provide(
			serviceconfig.NewRuntimeConfig,
			serviceconfig.NewAuthSettings,
			serviceconfig.NewRBACSettings,
			serviceconfig.NewEntSettings,
			serviceconfig.NewRateLimitSettings,
			serviceconfig.NewHTTPSettings,
			serviceconfig.NewResourceSettings,
		),
		validation.Module,
		authfeature.Module,
		permissionfeature.Module,
		rolefeature.Module,
		userfeature.Module,
		Module,
		fx.Invoke(func(*userhttp.UserController) {}),
	)
	require.NoError(t, err)
}

func TestSharedObservabilityProvidersStartFromServiceRuntimeConfig(t *testing.T) {
	serviceCfg := &serviceconfig.Config{
		Config: config.Config{
			App: config.AppConfig{Name: "configured-user-service", Environment: "test"},
			Observability: config.ObservabilityConfig{
				Metrics: config.MetricsConfig{Enabled: false},
				Tracing: config.TracingConfig{Enabled: false, SampleRatio: 1},
			},
		},
	}
	type observabilityProviders struct {
		fx.In

		Metrics *platformmetrics.Provider
		Tracing *platformtracing.Provider
	}
	var got observabilityProviders
	app := fxtest.New(t,
		fx.Supply(serviceconfig.NewRuntimeConfig(serviceCfg)),
		fx.Provide(platformmetrics.NewMetricsProvider, platformtracing.NewTracingProvider),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, got.Metrics)
	require.False(t, got.Metrics.Enabled())
	require.NotNil(t, got.Tracing)
	require.NoError(t, got.Tracing.Shutdown(context.Background()))
}
