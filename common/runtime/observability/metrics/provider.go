package metrics

import (
	"errors"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/aegiscore/common/runtime/config"
)

var (
	// ErrNilCollector 表示调用方尝试注册空 collector。
	ErrNilCollector = errors.New("metrics collector is nil")
)

// Options 描述构造 Prometheus metrics provider 所需的跨服务运行时输入。
type Options struct {
	Config      config.MetricsConfig
	ServiceName string
	Environment string
}

// Provider 持有 metrics runtime 的 Prometheus registry、registerer 和 gatherer。
type Provider struct {
	enabled    bool
	registry   *prometheus.Registry
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer
}

// NewProvider 基于配置创建 Prometheus metrics provider。
func NewProvider(opts Options) (*Provider, error) {
	serviceName := strings.TrimSpace(opts.ServiceName)
	if serviceName == "" {
		return nil, errors.New("metrics service name is required")
	}
	environment := strings.TrimSpace(opts.Environment)
	if environment == "" {
		return nil, errors.New("metrics deployment environment is required")
	}

	if !opts.Config.Enabled {
		return &Provider{enabled: false}, nil
	}

	registry := prometheus.NewRegistry()
	provider := &Provider{
		enabled:  true,
		registry: registry,
		registerer: prometheus.WrapRegistererWith(prometheus.Labels{
			LabelService:     serviceName,
			LabelEnvironment: environment,
		}, registry),
		gatherer: registry,
	}
	if opts.Config.IncludeRuntime {
		if err := provider.registerRuntimeCollectors(); err != nil {
			return nil, err
		}
	}
	return provider, nil
}

// Enabled 返回 metrics runtime 是否启用。
func (p *Provider) Enabled() bool {
	return p != nil && p.enabled
}

// Registerer 返回 Prometheus registerer；禁用模式返回 nil。
func (p *Provider) Registerer() prometheus.Registerer {
	if !p.Enabled() {
		return nil
	}
	return p.registerer
}

// Gatherer 返回 Prometheus gatherer；禁用模式返回 nil。
func (p *Provider) Gatherer() prometheus.Gatherer {
	if !p.Enabled() {
		return nil
	}
	return p.gatherer
}

// Register 注册 collector，并将重复注册视为成功。
func (p *Provider) Register(collector prometheus.Collector) error {
	if !p.Enabled() {
		return nil
	}
	if collector == nil {
		return ErrNilCollector
	}
	if err := p.registerer.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			return nil
		}
		return fmt.Errorf("register metrics collector: %w", err)
	}
	return nil
}

// MustRegister 注册 collector；只有非重复注册错误会触发 panic。
func (p *Provider) MustRegister(collector prometheus.Collector) {
	if err := p.Register(collector); err != nil {
		panic(err)
	}
}
