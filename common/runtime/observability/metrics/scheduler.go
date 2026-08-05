package metrics

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/aegiscore/common/runtime/scheduler"
)

const (
	schedulerJobsMetricName         = "aegiscore_scheduler_jobs_total"
	schedulerJobDurationMetricName  = "aegiscore_scheduler_job_duration_seconds"
	schedulerJobsMetricHelp         = "Total number of scheduler job runtime events by fixed scheduler job, event, status, and reason."
	schedulerJobDurationMetricHelp  = "Duration of actual scheduler job executions in seconds, excluding concurrency and lock waits."
	schedulerEventTriggered         = "triggered"
	schedulerEventStarted           = "started"
	schedulerEventCompleted         = "completed"
	schedulerEventFailed            = "failed"
	schedulerEventSkipped           = "skipped"
	schedulerEventLockRenewFailed   = "lock_renew_failed"
	schedulerStatusSuccess          = "success"
	schedulerStatusFailure          = "failure"
	schedulerStatusSkipped          = "skipped"
	schedulerReasonNone             = "none"
	defaultSchedulerDurationBuckets = 0
)

// SchedulerMetricsOptions 配置 scheduler Prometheus adapter。
type SchedulerMetricsOptions struct {
	DurationBuckets []float64
}

// schedulerMetrics 只接受固定 job key 与枚举事件，避免把 spec、错误或锁 key 写入 label。
type schedulerMetrics struct {
	jobs     *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewSchedulerMetrics 构造 scheduler.Metrics 的 Prometheus adapter。
func NewSchedulerMetrics(provider *Provider, opts SchedulerMetricsOptions) scheduler.Metrics {
	if provider == nil || !provider.Enabled() {
		return scheduler.NopMetrics{}
	}
	buckets := opts.DurationBuckets
	if len(buckets) == defaultSchedulerDurationBuckets {
		buckets = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}
	}
	recorder := &schedulerMetrics{
		jobs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: schedulerJobsMetricName,
			Help: schedulerJobsMetricHelp,
		}, []string{LabelSchedulerJob, LabelEvent, LabelStatus, LabelReason}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    schedulerJobDurationMetricName,
			Help:    schedulerJobDurationMetricHelp,
			Buckets: buckets,
		}, []string{LabelSchedulerJob, LabelStatus}),
	}
	provider.MustRegister(recorder.jobs)
	provider.MustRegister(recorder.duration)
	return recorder
}

// JobTriggered 记录 triggered/success/none 事件。
func (m *schedulerMetrics) JobTriggered(jobKey string) {
	m.record(jobKey, schedulerEventTriggered, schedulerStatusSuccess, schedulerReasonNone)
}

// JobStarted 记录 started/success/none 事件。
func (m *schedulerMetrics) JobStarted(jobKey string) {
	m.record(jobKey, schedulerEventStarted, schedulerStatusSuccess, schedulerReasonNone)
}

// JobCompleted 记录 completed 事件和 success duration。
func (m *schedulerMetrics) JobCompleted(jobKey string, duration time.Duration) {
	jobKey = normalizeRuntimeLabelValue(jobKey)
	m.record(jobKey, schedulerEventCompleted, schedulerStatusSuccess, schedulerReasonNone)
	m.duration.WithLabelValues(jobKey, schedulerStatusSuccess).Observe(duration.Seconds())
}

// JobFailed 记录 failed 事件和 failure duration。
func (m *schedulerMetrics) JobFailed(jobKey string, duration time.Duration) {
	jobKey = normalizeRuntimeLabelValue(jobKey)
	m.record(jobKey, schedulerEventFailed, schedulerStatusFailure, schedulerReasonNone)
	m.duration.WithLabelValues(jobKey, schedulerStatusFailure).Observe(duration.Seconds())
}

// JobSkipped 记录 skipped 事件及调用方提供的固定 reason 枚举。
func (m *schedulerMetrics) JobSkipped(jobKey string, reason string) {
	m.record(jobKey, schedulerEventSkipped, schedulerStatusSkipped, normalizeRuntimeLabelValue(reason))
}

// JobLockRenewFailed 记录独立的 lock_renew_failed 失败事件。
func (m *schedulerMetrics) JobLockRenewFailed(jobKey string) {
	m.record(jobKey, schedulerEventLockRenewFailed, schedulerStatusFailure, schedulerReasonNone)
}

// record 写入统一低基数的 scheduler event counter。
func (m *schedulerMetrics) record(jobKey string, event string, status string, reason string) {
	m.jobs.WithLabelValues(normalizeRuntimeLabelValue(jobKey), event, status, normalizeRuntimeLabelValue(reason)).Inc()
}

// normalizeRuntimeLabelValue 为意外空值提供稳定占位，不派生新的高基数 label。
func normalizeRuntimeLabelValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
