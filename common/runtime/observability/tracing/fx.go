package tracing

import (
	"context"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
)

// FxParams 描述 Fx 构造 tracing provider 所需依赖。
type FxParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
}

// NewFxProvider 从共享配置构造 tracing provider，并注册关闭生命周期。
func NewFxProvider(params FxParams) (*Provider, error) {
	provider, err := NewProvider(context.Background(), Options{
		Config:      params.Config.Observability.Tracing,
		ServiceName: params.Config.App.Name,
		Environment: params.Config.App.Environment,
	})
	if err != nil {
		return nil, err
	}
	params.Lifecycle.Append(fx.Hook{
		OnStop: provider.Shutdown,
	})
	return provider, nil
}
