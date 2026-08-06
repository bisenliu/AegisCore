package metrics

import (
	"errors"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	statusRunningMetricName   = "aegiscore_runtime_component_running"
	statusLastErrorMetricName = "aegiscore_runtime_component_last_error"
	statusRunningMetricHelp   = "Whether the runtime component reports itself as running."
	statusLastErrorMetricHelp = "Whether the runtime component has a last error."
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
