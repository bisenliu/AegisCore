package providers

import (
	"testing"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/validation"
	authfeature "github.com/aegiscore/user-service/internal/features/auth"
	userfeature "github.com/aegiscore/user-service/internal/features/user"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func TestModuleResolvesServiceLevelProviders(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		validation.Module,
		authfeature.Module,
		userfeature.Module,
		Module,
		fx.Invoke(func(*userhttp.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp error = %v", err)
	}
}
