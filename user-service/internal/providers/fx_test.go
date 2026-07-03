package providers

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/validation"
	authfeature "github.com/aegiscore/user-service/internal/features/auth"
	permissionfeature "github.com/aegiscore/user-service/internal/features/permission"
	userfeature "github.com/aegiscore/user-service/internal/features/user"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

func TestModuleResolvesServiceLevelProviders(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{
			App: config.AppConfig{Name: "configured-user-service", Environment: "test"},
			Auth: config.AuthConfig{
				PasswordKDF: config.PasswordKDFConfig{Argon2Concurrency: 1, Argon2QueueSize: 1},
			},
			Observability: config.ObservabilityConfig{
				Tracing: config.TracingConfig{Enabled: true, SampleRatio: 1, Exporter: "none"},
			},
		}, zap.NewNop()),
		validation.Module,
		authfeature.Module,
		permissionfeature.Module,
		userfeature.Module,
		Module,
		fx.Invoke(func(*userhttp.UserController) {}),
	)
	require.NoError(t, err)
}
