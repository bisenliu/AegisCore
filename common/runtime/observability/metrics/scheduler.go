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
	schedulerJobsMetricHelp         = "Total number of scheduler job lifecycle events."
	schedulerJobDurationMetricHelp  = "Duration of scheduler job executions in seconds."
	schedulerEventRegistered        = "registered"
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

var schedulerDurationBuckets = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

// SchedulerMetricsOptions 配置 scheduler Prometheus adapter。
type SchedulerMetricsOptions struct {
	DurationBuckets []float64
}

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
		buckets = schedulerDurationBuckets
	}
	recorder := &schedulerMetrics{
		jobs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: schedulerJobsMetricName,
			Help: schedulerJobsMetricHelp,
		}, []string{LabelJob, LabelEvent, LabelStatus, LabelReason}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    schedulerJobDurationMetricName,
			Help:    schedulerJobDurationMetricHelp,
			Buckets: buckets,
		}, []string{LabelJob, LabelStatus}),
	}
	provider.MustRegister(recorder.jobs)
	provider.MustRegister(recorder.duration)
	return recorder
}

func (m *schedulerMetrics) JobRegistered(jobKey string) {
	m.record(jobKey, schedulerEventRegistered, schedulerStatusSuccess, schedulerReasonNone)
}

func (m *schedulerMetrics) JobTriggered(jobKey string) {
	m.record(jobKey, schedulerEventTriggered, schedulerStatusSuccess, schedulerReasonNone)
}

func (m *schedulerMetrics) JobStarted(jobKey string) {
	m.record(jobKey, schedulerEventStarted, schedulerStatusSuccess, schedulerReasonNone)
}

func (m *schedulerMetrics) JobCompleted(jobKey string, duration time.Duration) {
	jobKey = normalizeRuntimeLabelValue(jobKey)
	m.record(jobKey, schedulerEventCompleted, schedulerStatusSuccess, schedulerReasonNone)
	m.duration.WithLabelValues(jobKey, schedulerStatusSuccess).Observe(duration.Seconds())
}

func (m *schedulerMetrics) JobFailed(jobKey string, duration time.Duration) {
	jobKey = normalizeRuntimeLabelValue(jobKey)
	m.record(jobKey, schedulerEventFailed, schedulerStatusFailure, schedulerReasonNone)
	m.duration.WithLabelValues(jobKey, schedulerStatusFailure).Observe(duration.Seconds())
}

func (m *schedulerMetrics) JobSkipped(jobKey string, reason string) {
	m.record(jobKey, schedulerEventSkipped, schedulerStatusSkipped, normalizeRuntimeLabelValue(reason))
}

func (m *schedulerMetrics) JobLockRenewFailed(jobKey string) {
	m.record(jobKey, schedulerEventLockRenewFailed, schedulerStatusFailure, schedulerReasonNone)
}

func (m *schedulerMetrics) record(jobKey string, event string, status string, reason string) {
	m.jobs.WithLabelValues(normalizeRuntimeLabelValue(jobKey), event, status, normalizeRuntimeLabelValue(reason)).Inc()
}

func normalizeRuntimeLabelValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
