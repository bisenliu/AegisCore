package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	commonconfig "github.com/aegiscore/common/runtime/config"
	commonnacos "github.com/aegiscore/common/runtime/config/nacos"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// newConfigCommand 创建 user-service 配置检查命令组。
func newConfigCommand(loadConfig configLoader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect user-service runtime configuration sources",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newConfigValidateCommand(loadConfig), newConfigRenderCommand(loadConfig), newConfigSourcesCommand())
	return cmd
}

// newConfigValidateCommand 创建只校验配置、不启动运行时资源的命令。
func newConfigValidateCommand(loadConfig configLoader) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate Nacos runtime configuration without starting runtime resources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := loadConfig(cmd.Context()); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "config valid")
			return err
		},
	}
}

// newConfigRenderCommand 创建渲染 effective settings 的命令，输出前必须使用 user-service 脱敏策略。
func newConfigRenderCommand(loadConfig configLoader) *cobra.Command {
	return &cobra.Command{
		Use:   "render",
		Short: "Render merged runtime configuration with sensitive values redacted",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			loaded, err := loadConfig(cmd.Context())
			if err != nil {
				return err
			}
			settings, err := loaded.EffectiveSettings()
			if err != nil {
				return err
			}
			// CLI 只渲染 user-service 自己声明的脱敏结果，避免 common 隐式持有服务私有 secret 路径。
			rendered, err := commonconfig.RenderYAML(serviceconfig.RedactEffectiveSettings(settings))
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(rendered)
			return err
		},
	}
}

// newConfigSourcesCommand 创建展示 Nacos source 顺序和环境选择的命令。
func newConfigSourcesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "Show Nacos runtime configuration source order",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := commonnacos.LoadEnv()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			lines := []string{
				"config_provider: nacos",
				fmt.Sprintf("config_service: %s", env.Service),
				fmt.Sprintf("config_namespace: %s", env.Namespace),
				fmt.Sprintf("config_group: %s", env.Group),
				fmt.Sprintf("config_data_ids: %s", commonconfig.SourceMetadata{DataIDs: env.DataIDs}.DataIDsCSV()),
				fmt.Sprintf("config_timeout: %s", env.Timeout.Round(time.Millisecond)),
			}
			if env.Username != "" || env.Password != "" {
				lines = append(lines, "config_auth: enabled")
			}
			for _, line := range lines {
				if _, err := fmt.Fprintln(out, line); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
