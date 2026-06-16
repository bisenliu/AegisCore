package providers

import (
	"testing"

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
		fx.Supply(&config.Config{}, zap.NewNop()),
		validation.Module,
		authfeature.Module,
		permissionfeature.Module,
		userfeature.Module,
		Module,
		fx.Invoke(func(*userhttp.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp error = %v", err)
	}
}
