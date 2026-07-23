package metrics

import (
	"errors"

	"github.com/aegiscore/common/runtime/config"
)

// NewMetricsProvider 从共享配置构造 metrics provider，不挂载 HTTP 路由。
func NewMetricsProvider(cfg *config.Config) (*Provider, error) {
	if cfg == nil {
		return nil, errors.New("metrics config is required")
	}
	return NewProvider(Options{
		Config:      cfg.Observability.Metrics,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})
}
