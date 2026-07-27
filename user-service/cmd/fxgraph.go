package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/internal/bootstrap"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

const defaultFxGraphOutputPath = "./docs/fx-dependency-graph.dot"

type fxGraphWriter func(path string, opts ...fx.Option) (string, error)

func newFxGraphCommand(writer fxGraphWriter, loadConfig configLoader) *cobra.Command {
	var outputPath string
	cmd := &cobra.Command{
		Use:   "fxgraph",
		Short: "Generate the user-service Fx dependency graph",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFxGraphCommand(cmd.Context(), outputPath, writer, loadConfig)
		},
	}
	cmd.Flags().StringVar(&outputPath, "output", defaultFxGraphOutputPath, "path to write DOT dependency graph")
	return cmd
}

func runFxGraphCommand(ctx context.Context, outputPath string, writer fxGraphWriter, loadConfig configLoader) error {
	if outputPath == "" {
		return fmt.Errorf("fx graph output path is required")
	}
	if writer == nil {
		return fmt.Errorf("fx graph writer is required")
	}
	loaded, err := loadConfig(ctx)
	if err != nil {
		return err
	}
	_, err = writer(outputPath, fxGraphOptions(loaded.Config)...)
	if err != nil {
		return err
	}
	fmt.Printf("Fx dependency graph generated: %s\n", outputPath)
	return nil
}

func fxGraphOptions(cfg *serviceconfig.Config) []fx.Option {
	return bootstrap.AppOptions(
		cfg,
		// Fx 分类：开发工具 - 仅使用无运行时激活副作用的 wiring graph。
		bootstrap.WiringModule,
	)
}
