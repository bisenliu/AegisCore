package bootstrap

import (
	"testing"

	"github.com/aegiscore/common/runtime/config"
	commoninfra "github.com/aegiscore/common/runtime/infrastructure"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/controller"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func TestUserServiceModuleResolvesSharedValidationDependency(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		UserServiceModule,
		fx.Invoke(func(*validation.Validator, *controller.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp error = %v", err)
	}
}

func TestUserServiceModuleIncludesSharedTimezoneDependency(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		UserServiceModule,
		fx.Invoke(func(*validation.Validator, *controller.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp with timezone module error = %v", err)
	}
}

func TestAppWiresCommonDependenciesExplicitly(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(commoninfra.ConfigPath("../../configs/config.yaml")),
		fx.Provide(
			commoninfra.NewConfig,
			commoninfra.NewLogger,
		),
		UserServiceModule,
		fx.Invoke(func(*config.Config, *zap.Logger, *controller.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp with explicit common providers error = %v", err)
	}
}
