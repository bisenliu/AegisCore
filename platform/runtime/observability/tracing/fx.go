package tracing

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/fx"

	"github.com/aegiscore/platform/runtime/config"
)

// NewTracingProvider 从共享配置构造 tracing provider，并注册 Fx 生命周期。
//
// 该 constructor 只在 Fx graph 构造阶段创建可注入的 provider facade，不会连接 OTLP endpoint 或
// 启动 batch processor。真实 SDK provider 会在 OnStart 中创建，并在 OnStop 或 Fx rollback 中关闭。
func NewTracingProvider(lifecycle fx.Lifecycle, cfg *config.Config) (*Provider, error) {
	return newTracingProvider(lifecycle, cfg, newOTLPExporter)
}

// newTracingProvider 支持传入自定义的数据导出函数，方便测试时模拟启动失败、配置禁用或正常关闭等不同场景；生产环境下默认使用标准的 OpenTelemetry 协议向监控收集端上报数据。
func newTracingProvider(lifecycle fx.Lifecycle, cfg *config.Config, createExporter exporterFactory) (*Provider, error) {
	if cfg == nil {
		return nil, errors.New("tracing config is required")
	}
	opts := Options{
		Config:      cfg.Observability.Tracing,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	}
	if err := validateFxOptions(opts); err != nil {
		return nil, err
	}
	provider, sampler, err := newUnstartedProvider(opts)
	if err != nil {
		return nil, err
	}
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return provider.start(ctx, opts.Config, sampler, createExporter) },
		OnStop:  provider.Shutdown,
	})
	return provider, nil
}

// validateFxOptions 在 Fx hook 注册前验证 tracing 启动所需配置。
//
// Fx graph 构造阶段必须尽早暴露服务名、环境、采样率和 enabled endpoint 错误，避免错误延迟到
// Redis、Gin、Ent 或 HTTP server 初始化阶段才出现。
func validateFxOptions(opts Options) error {
	if strings.TrimSpace(opts.ServiceName) == "" {
		return errors.New("tracing service name is required")
	}
	if strings.TrimSpace(opts.Environment) == "" {
		return errors.New("tracing deployment environment is required")
	}
	if opts.Config.SampleRatio < 0 || opts.Config.SampleRatio > 1 {
		return errors.New("tracing sample ratio must be between 0 and 1")
	}
	if opts.Config.Enabled && strings.TrimSpace(opts.Config.OTLPEndpoint) == "" {
		return errors.New("otlp tracing endpoint is required")
	}
	return nil
}
