package metrics

import (
	"errors"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/aegiscore/common/runtime/workerpool"
)

const (
	workerpoolTasksMetricName   = "aegiscore_workerpool_tasks_total"
	workerpoolQueuedMetricName  = "aegiscore_workerpool_queued"
	workerpoolRunningMetricName = "aegiscore_workerpool_running"
	workerpoolWaitingMetricName = "aegiscore_workerpool_waiting"
	workerpoolTasksMetricHelp   = "Total number of workerpool tasks by fixed pool and lifecycle event."
	workerpoolQueuedMetricHelp  = "Current number of queued workerpool tasks."
	workerpoolRunningMetricHelp = "Current number of running workerpool tasks."
	workerpoolWaitingMetricHelp = "Current number of submitters waiting for workerpool workers."
	workerpoolEventSubmitted    = "submitted"
	workerpoolEventRejected     = "rejected"
	workerpoolEventStarted      = "started"
	workerpoolEventCompleted    = "completed"
	workerpoolEventFailed       = "failed"
	workerpoolEventPanicked     = "panicked"
)

// WorkerpoolCollectorOptions 配置后台任务池指标 collector。
type WorkerpoolCollectorOptions struct {
	Pool   string
	Source workerpool.StatsSource
}

// WorkerpoolCollector 从 workerpool.Stats 快照导出指标。
type WorkerpoolCollector struct {
	pool   string
	source workerpool.StatsSource

	tasks   *prometheus.Desc
	queued  *prometheus.Desc
	running *prometheus.Desc
	waiting *prometheus.Desc
}

// NewWorkerpoolCollector 构造后台任务池指标 collector。
func NewWorkerpoolCollector(opts WorkerpoolCollectorOptions) (*WorkerpoolCollector, error) {
	if opts.Source == nil {
		return nil, errors.New("workerpool metrics source is required")
	}
	pool := strings.TrimSpace(opts.Pool)
	if pool == "" {
		pool = strings.TrimSpace(opts.Source.Stats().Name)
	}
	if pool == "" {
		return nil, errors.New("workerpool metrics pool is required")
	}

	return &WorkerpoolCollector{
		pool:    pool,
		source:  opts.Source,
		tasks:   prometheus.NewDesc(workerpoolTasksMetricName, workerpoolTasksMetricHelp, []string{LabelPool, LabelEvent}, nil),
		queued:  prometheus.NewDesc(workerpoolQueuedMetricName, workerpoolQueuedMetricHelp, []string{LabelPool}, nil),
		running: prometheus.NewDesc(workerpoolRunningMetricName, workerpoolRunningMetricHelp, []string{LabelPool}, nil),
		waiting: prometheus.NewDesc(workerpoolWaitingMetricName, workerpoolWaitingMetricHelp, []string{LabelPool}, nil),
	}, nil
}

// Describe 实现 prometheus.Collector。
func (c *WorkerpoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.tasks
	ch <- c.queued
	ch <- c.running
	ch <- c.waiting
}

// Collect 实现 prometheus.Collector。
func (c *WorkerpoolCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.source.Stats()
	ch <- prometheus.MustNewConstMetric(c.tasks, prometheus.CounterValue, float64(stats.Submitted), c.pool, workerpoolEventSubmitted)
	ch <- prometheus.MustNewConstMetric(c.tasks, prometheus.CounterValue, float64(stats.Rejected), c.pool, workerpoolEventRejected)
	ch <- prometheus.MustNewConstMetric(c.tasks, prometheus.CounterValue, float64(stats.Started), c.pool, workerpoolEventStarted)
	ch <- prometheus.MustNewConstMetric(c.tasks, prometheus.CounterValue, float64(stats.Completed), c.pool, workerpoolEventCompleted)
	ch <- prometheus.MustNewConstMetric(c.tasks, prometheus.CounterValue, float64(stats.Failed), c.pool, workerpoolEventFailed)
	ch <- prometheus.MustNewConstMetric(c.tasks, prometheus.CounterValue, float64(stats.Panicked), c.pool, workerpoolEventPanicked)
	ch <- prometheus.MustNewConstMetric(c.queued, prometheus.GaugeValue, float64(stats.Queued), c.pool)
	ch <- prometheus.MustNewConstMetric(c.running, prometheus.GaugeValue, float64(stats.Running), c.pool)
	ch <- prometheus.MustNewConstMetric(c.waiting, prometheus.GaugeValue, float64(stats.Waiting), c.pool)
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
