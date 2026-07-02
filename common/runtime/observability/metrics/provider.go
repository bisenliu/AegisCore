package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	io_prometheus_client "github.com/prometheus/client_model/go"

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
	gatherMu   sync.Mutex
	gatherCtx  context.Context
}

// ContextCollector 定义可消费 scrape context 的 Prometheus collector。
type ContextCollector interface {
	prometheus.Collector
	CollectContext(ctx context.Context, ch chan<- prometheus.Metric)
}

type contextCollectorWrapper struct {
	provider  *Provider
	collector ContextCollector
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

// GatherContext 使用调用方 context 采集支持 context 的 collector。
func (p *Provider) GatherContext(ctx context.Context) ([]*io_prometheus_client.MetricFamily, error) {
	if !p.Enabled() {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	p.gatherMu.Lock()
	defer p.gatherMu.Unlock()
	p.gatherCtx = ctx
	defer func() { p.gatherCtx = nil }()
	return p.gatherer.Gather()
}

// HTTPHandler 返回使用 HTTP request context 采集指标的 handler。
func (p *Provider) HTTPHandler(opts promhttp.HandlerOpts) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !p.Enabled() {
			http.NotFound(w, r)
			return
		}
		gatherer := prometheus.GathererFunc(func() ([]*io_prometheus_client.MetricFamily, error) {
			return p.GatherContext(r.Context())
		})
		promhttp.HandlerFor(gatherer, opts).ServeHTTP(w, r)
	})
}

// Register 注册 collector，并将重复注册视为成功。
func (p *Provider) Register(collector prometheus.Collector) error {
	if !p.Enabled() {
		return nil
	}
	if collector == nil {
		return ErrNilCollector
	}
	if contextCollector, ok := collector.(ContextCollector); ok {
		collector = contextCollectorWrapper{provider: p, collector: contextCollector}
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

func (w contextCollectorWrapper) Describe(ch chan<- *prometheus.Desc) {
	w.collector.Describe(ch)
}

func (w contextCollectorWrapper) Collect(ch chan<- prometheus.Metric) {
	ctx := w.provider.currentGatherContext()
	w.collector.CollectContext(ctx, ch)
}

func (p *Provider) currentGatherContext() context.Context {
	if p == nil || p.gatherCtx == nil {
		return context.Background()
	}
	return p.gatherCtx
}
