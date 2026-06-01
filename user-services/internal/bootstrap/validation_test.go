package bootstrap

import (
	"testing"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/controller"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func TestModuleResolvesSharedValidationDependency(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		Module,
		fx.Invoke(func(*validation.Validator, *controller.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp error = %v", err)
	}
}

func TestModuleIncludesSharedTimezoneDependency(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		Module,
		fx.Invoke(func(*validation.Validator, *controller.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp with timezone module error = %v", err)
	}
}
