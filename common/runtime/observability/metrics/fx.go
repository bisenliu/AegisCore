package metrics

import (
	"errors"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
)

// FxParams 描述 Fx 构造 metrics provider 所需依赖。
type FxParams struct {
	fx.In

	Config *config.Config
}

// NewFxProvider 从共享配置构造 metrics provider，不挂载 HTTP 路由。
func NewFxProvider(params FxParams) (*Provider, error) {
	if params.Config == nil {
		return nil, errors.New("metrics config is required")
	}
	return NewProvider(Options{
		Config:      params.Config.Observability.Metrics,
		ServiceName: params.Config.App.Name,
		Environment: params.Config.App.Environment,
	})
}
