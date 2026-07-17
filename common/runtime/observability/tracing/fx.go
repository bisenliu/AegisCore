package tracing

import (
	"context"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
)

// NewFxProvider 从共享配置构造 tracing provider，并注册关闭生命周期。
func NewFxProvider(lifecycle fx.Lifecycle, cfg *config.Config) (*Provider, error) {
	provider, err := NewProvider(context.Background(), Options{
		Config:      cfg.Observability.Tracing,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
	if err != nil {
		return nil, err
	}
	lifecycle.Append(fx.StopHook(provider.Shutdown))
	return provider, nil
}
