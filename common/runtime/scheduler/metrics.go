package scheduler

import "time"

// Metrics 定义定时任务可观测性上报接口，便于后续接入 Prometheus 等监控系统。
type Metrics interface {
	JobRegistered(jobKey string)
	JobTriggered(jobKey string)
	JobStarted(jobKey string)
	JobCompleted(jobKey string, duration time.Duration)
	JobFailed(jobKey string, duration time.Duration)
	JobSkipped(jobKey, reason string)
	JobLockRenewFailed(jobKey string)
}

// NopMetrics 是默认空实现，调用方未接入监控时保持零副作用。
type NopMetrics struct{}

func (NopMetrics) JobRegistered(string)               {}
func (NopMetrics) JobTriggered(string)                {}
func (NopMetrics) JobStarted(string)                  {}
func (NopMetrics) JobCompleted(string, time.Duration) {}
func (NopMetrics) JobFailed(string, time.Duration)    {}
func (NopMetrics) JobSkipped(string, string)          {}
func (NopMetrics) JobLockRenewFailed(string)          {}
