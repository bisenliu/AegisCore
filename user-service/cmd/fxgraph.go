package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/user-service/internal/bootstrap"
)

const defaultFxGraphOutputPath = "./docs/fx-dependency-graph.dot"

type fxGraphWriter func(path string, opts ...fx.Option) (string, error)

func newFxGraphCommand(writer fxGraphWriter) *cobra.Command {
	var configPath string
	var outputPath string
	cmd := &cobra.Command{
		Use:   "fxgraph",
		Short: "Generate the user-service Fx dependency graph",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runFxGraphCommand(configPath, outputPath, writer)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./configs/config.yaml", "path to YAML configuration file")
	cmd.Flags().StringVar(&outputPath, "output", defaultFxGraphOutputPath, "path to write DOT dependency graph")
	return cmd
}

func runFxGraphCommand(configPath string, outputPath string, writer fxGraphWriter) error {
	if outputPath == "" {
		return fmt.Errorf("fx graph output path is required")
	}
	if writer == nil {
		return fmt.Errorf("fx graph writer is required")
	}
	_, err := writer(outputPath,
		// Fx 分类：开发工具 - 依赖图构建所需的启动输入与日志替身。
		fx.Supply(config.ConfigPath(configPath), zap.NewNop()),
		// Fx 分类：开发工具 - 依赖图构建所需的基础配置 provider。
		fx.Provide(config.NewConfig),
		// Fx 分类：开发工具 - 复用正式 composition root 校验完整依赖图。
		bootstrap.AppModule,
	)
	if err != nil {
		return err
	}
	fmt.Printf("Fx dependency graph generated: %s\n", outputPath)
	return nil
}
