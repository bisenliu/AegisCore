package metrics

import (
	"errors"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/aegiscore/platform/runtime/workerpool"
)

const (
	// workerpoolTasksMetricName 是工作池任务生命周期事件累计 Counter 指标名。
	workerpoolTasksMetricName = "aegiscore_workerpool_tasks_total"
	// workerpoolQueuedMetricName 是工作池当前队列中排队任务数 Gauge 指标名。
	workerpoolQueuedMetricName = "aegiscore_workerpool_queued"
	// workerpoolRunningMetricName 是工作池当前正在执行的任务数 Gauge 指标名。
	workerpoolRunningMetricName = "aegiscore_workerpool_running"
	// workerpoolWaitingMetricName 是因工作池饱和正在等待空闲 worker 的提交者数量 Gauge 指标名。
	workerpoolWaitingMetricName = "aegiscore_workerpool_waiting"
	// workerpoolTasksMetricHelp 是工作池任务事件指标说明。
	workerpoolTasksMetricHelp = "Total number of workerpool tasks by fixed pool and lifecycle event."
	// workerpoolQueuedMetricHelp 是工作池队列排队任务数指标说明。
	workerpoolQueuedMetricHelp = "Current number of queued workerpool tasks."
	// workerpoolRunningMetricHelp 是工作池正在执行任务数指标说明。
	workerpoolRunningMetricHelp = "Current number of running workerpool tasks."
	// workerpoolWaitingMetricHelp 是工作池等待提交者数量指标说明。
	workerpoolWaitingMetricHelp = "Current number of submitters waiting for workerpool workers."
	// workerpoolEventSubmitted 表示任务提交成功事件。
	workerpoolEventSubmitted = "submitted"
	// workerpoolEventRejected 表示任务因队列已满或工作池关闭被拒绝事件。
	workerpoolEventRejected = "rejected"
	// workerpoolEventStarted 表示 worker 开始处理任务事件。
	workerpoolEventStarted = "started"
	// workerpoolEventCompleted 表示任务顺利执行完成事件。
	workerpoolEventCompleted = "completed"
	// workerpoolEventFailed 表示任务执行返回错误事件。
	workerpoolEventFailed = "failed"
	// workerpoolEventPanicked 表示任务执行发生 panic 异常事件。
	workerpoolEventPanicked = "panicked"
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

// boolFloat 将开启/关闭状态（true/false）转换为数字 1 或 0，方便监控系统进行图表绘制。
func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
