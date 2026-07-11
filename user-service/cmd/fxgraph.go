package main

import (
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/user-service/internal/bootstrap"
)

const defaultFxGraphOutputPath = "./docs/fx-dependency-graph.dot"

type fxGraphWriter func(path string, opts ...fx.Option) (string, error)

func runFxGraphCommand(configPath string, outputPath string, writer fxGraphWriter) error {
	if outputPath == "" {
		return fmt.Errorf("fx graph output path is required")
	}
	if writer == nil {
		return fmt.Errorf("fx graph writer is required")
	}
	_, err := writer(outputPath,
		fx.Supply(config.ConfigPath(configPath), zap.NewNop()),
		fx.Provide(config.NewConfig),
		bootstrap.AppModule,
	)
	if err != nil {
		return err
	}
	fmt.Printf("Fx dependency graph generated: %s\n", outputPath)
	return nil
}
