package metrics

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	io_prometheus_client "github.com/prometheus/client_model/go"
)

// ContextCollector 定义可消费 scrape context 的 Prometheus collector。
type ContextCollector interface {
	prometheus.Collector
	CollectContext(ctx context.Context, ch chan<- prometheus.Metric)
}

type contextCollectorWrapper struct {
	provider  *Provider
	collector ContextCollector
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
