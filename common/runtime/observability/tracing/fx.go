package tracing

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
)

// NewFxProvider 从共享配置构造 tracing provider，并注册关闭生命周期。
func NewFxProvider(lifecycle fx.Lifecycle, cfg *config.Config) (*Provider, error) {
	return newFxProvider(lifecycle, cfg, newOTLPExporter)
}

func newFxProvider(lifecycle fx.Lifecycle, cfg *config.Config, createExporter exporterFactory) (*Provider, error) {
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
	provider, err := newProvider(context.Background(), opts, createExporter)
	if err != nil {
		return nil, err
	}
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error { return nil },
		OnStop:  provider.Shutdown,
	})
	return provider, nil
}

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
