package metrics

import (
	"errors"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	statusRunningMetricName    = "aegiscore_runtime_component_running"
	statusLastErrorMetricName  = "aegiscore_runtime_component_last_error"
	statusRunningMetricHelp    = "Whether the runtime component reports itself as running."
	statusLastErrorMetricHelp  = "Whether the runtime component has a last error."
	casbinReloadsMetricName    = "aegiscore_casbin_policy_reloads_total"
	casbinReloadLastMetricName = "aegiscore_casbin_policy_reload_last_success"
	casbinReloadsMetricHelp    = "Total number of Casbin policy reload results by status."
	casbinReloadLastMetricHelp = "Whether the latest Casbin policy reload succeeded."
	// StatusSuccess 表示运行时操作成功。
	StatusSuccess = "success"
	// StatusFailure 表示运行时操作失败。
	StatusFailure = "failure"
)

// ComponentStatusSource 暴露运行时组件只读状态。
type ComponentStatusSource interface {
	Running() bool
	LastError() error
}

// ComponentStatusCollectorOptions 配置运行时组件状态指标 collector。
type ComponentStatusCollectorOptions struct {
	Resource string
	Source   ComponentStatusSource
}

// ComponentStatusCollector 导出运行时组件 running 和 last error 状态。
type ComponentStatusCollector struct {
	resource string
	source   ComponentStatusSource
	running  *prometheus.Desc
	lastErr  *prometheus.Desc
}

// ReloadMetrics 记录 Casbin policy reload 结果。
type ReloadMetrics interface {
	ReloadSucceeded()
	ReloadFailed()
	SetLastStatus(success bool)
}

type nopReloadMetrics struct{}

type casbinPolicyReloadMetrics struct {
	reloads    *prometheus.CounterVec
	lastStatus prometheus.Gauge
}

// NopReloadMetrics 返回 policy reload metrics 的空实现。
func NopReloadMetrics() ReloadMetrics {
	return nopReloadMetrics{}
}

func (nopReloadMetrics) ReloadSucceeded()   {}
func (nopReloadMetrics) ReloadFailed()      {}
func (nopReloadMetrics) SetLastStatus(bool) {}

// NewComponentStatusCollector 构造运行时组件状态 collector。
func NewComponentStatusCollector(opts ComponentStatusCollectorOptions) (*ComponentStatusCollector, error) {
	resource := strings.TrimSpace(opts.Resource)
	if resource == "" {
		return nil, errors.New("component status metrics resource is required")
	}
	if opts.Source == nil {
		return nil, errors.New("component status metrics source is required")
	}
	return &ComponentStatusCollector{
		resource: resource,
		source:   opts.Source,
		running:  prometheus.NewDesc(statusRunningMetricName, statusRunningMetricHelp, []string{LabelResource}, nil),
		lastErr:  prometheus.NewDesc(statusLastErrorMetricName, statusLastErrorMetricHelp, []string{LabelResource}, nil),
	}, nil
}

// Describe 实现 prometheus.Collector。
func (c *ComponentStatusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.running
	ch <- c.lastErr
}

// Collect 实现 prometheus.Collector。
func (c *ComponentStatusCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.running, prometheus.GaugeValue, boolFloat(c.source.Running()), c.resource)
	ch <- prometheus.MustNewConstMetric(c.lastErr, prometheus.GaugeValue, boolFloat(c.source.LastError() != nil), c.resource)
}

// NewCasbinPolicyReloadMetrics 构造 Casbin policy reload Prometheus recorder。
func NewCasbinPolicyReloadMetrics(provider *Provider) ReloadMetrics {
	if provider == nil || !provider.Enabled() {
		return NopReloadMetrics()
	}
	recorder := &casbinPolicyReloadMetrics{
		reloads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: casbinReloadsMetricName,
			Help: casbinReloadsMetricHelp,
		}, []string{LabelStatus}),
		lastStatus: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: casbinReloadLastMetricName,
			Help: casbinReloadLastMetricHelp,
		}),
	}
	provider.MustRegister(recorder.reloads)
	provider.MustRegister(recorder.lastStatus)
	return recorder
}

func (m *casbinPolicyReloadMetrics) ReloadSucceeded() {
	m.reloads.WithLabelValues(StatusSuccess).Inc()
}

func (m *casbinPolicyReloadMetrics) ReloadFailed() {
	m.reloads.WithLabelValues(StatusFailure).Inc()
}

func (m *casbinPolicyReloadMetrics) SetLastStatus(success bool) {
	if success {
		m.lastStatus.Set(1)
		return
	}
	m.lastStatus.Set(0)
}
