package main

import (
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/fxgraph"
	"github.com/aegiscore/user-service/internal/bootstrap"
)

const defaultFxGraphOutputPath = "./docs/fx-dependency-graph.dot"

var writeFxGraph = fxgraph.WriteDOT

func runFxGraphCommand(configPath string, outputPath string) error {
	if outputPath == "" {
		return fmt.Errorf("fx graph output path is required")
	}
	_, err := writeFxGraph(outputPath,
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
