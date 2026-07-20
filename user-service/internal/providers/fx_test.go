package providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	"github.com/aegiscore/common/validation"
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
	}
	err := fx.ValidateApp(
		fx.Supply(serviceconfig.NewRuntimeConfig(serviceCfg), serviceCfg, zap.NewNop()),
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

		Metrics *commonmetrics.Provider
		Tracing *commontracing.Provider
	}
	var got observabilityProviders
	app := fxtest.New(t,
		fx.Supply(serviceconfig.NewRuntimeConfig(serviceCfg)),
		fx.Provide(commonmetrics.NewFxProvider, commontracing.NewFxProvider),
		fx.Populate(&got),
	)
	app.RequireStart()
	app.RequireStop()

	require.NotNil(t, got.Metrics)
	require.False(t, got.Metrics.Enabled())
	require.NotNil(t, got.Tracing)
	require.NoError(t, got.Tracing.Shutdown(context.Background()))
}
