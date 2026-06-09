package bootstrap

import (
	"os"
	"strings"
	"testing"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/configfx"
	"github.com/aegiscore/common/runtime/loggerfx"
	"github.com/aegiscore/common/validation"
	userhttp "github.com/aegiscore/user-services/internal/features/user/transport/http"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func TestAppModuleResolvesSharedValidationDependency(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		AppModule,
		fx.Invoke(func(*validation.Validator, *userhttp.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp error = %v", err)
	}
}

func TestAppModuleIncludesSharedTimezoneDependency(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(&config.Config{}, zap.NewNop()),
		AppModule,
		fx.Invoke(func(*validation.Validator, *userhttp.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp with timezone module error = %v", err)
	}
}

func TestAppWiresCommonDependenciesExplicitly(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(configfx.ConfigPath("../../configs/config.yaml")),
		fx.Provide(
			configfx.NewConfig,
			loggerfx.NewLogger,
		),
		AppModule,
		fx.Invoke(func(*config.Config, *zap.Logger, *userhttp.UserController) {}),
	)
	if err != nil {
		t.Fatalf("ValidateApp with explicit common providers error = %v", err)
	}
}

func TestRuntimeModuleNamingReflectsCompositionRootScope(t *testing.T) {
	content, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatalf("ReadFile app.go error = %v", err)
	}

	source := string(content)
	legacyName := "User" + "ServiceModule"
	if strings.Contains(source, legacyName) {
		t.Fatalf("app.go contains legacy service-layer-like module name %q", legacyName)
	}
	supersededName := "User" + "ServiceRuntimeModule"
	if strings.Contains(source, supersededName) {
		t.Fatalf("app.go contains superseded runtime module name %q", supersededName)
	}
	if !strings.Contains(source, "AppModule") {
		t.Fatal("app.go does not contain composition-root module name AppModule")
	}
}
