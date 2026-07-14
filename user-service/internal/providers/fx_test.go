package providers

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/validation"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authfeature "github.com/aegiscore/user-service/internal/features/auth"
	permissionfeature "github.com/aegiscore/user-service/internal/features/permission"
	rolefeature "github.com/aegiscore/user-service/internal/features/role"
	userfeature "github.com/aegiscore/user-service/internal/features/user"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
)

func TestModuleResolvesServiceLevelProviders(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{
			App: config.AppConfig{Name: "configured-user-service", Environment: "test"},
			Observability: config.ObservabilityConfig{
				Tracing: config.TracingConfig{Enabled: false, SampleRatio: 1},
			},
		}, &serviceconfig.Config{Auth: serviceconfig.AuthConfig{PasswordKDF: serviceconfig.PasswordKDFConfig{Argon2Concurrency: 1, Argon2QueueSize: 1}}}, zap.NewNop()),
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
