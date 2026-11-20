package scheduler

import "time"

// Metrics 定义定时任务可观测性上报接口，便于后续接入 Prometheus 等监控系统。
type Metrics interface {
	// JobTriggered 记录 cron 已触发一次固定 key 的任务。
	JobTriggered(jobKey string)
	// JobStarted 记录任务已通过全部准入阶段并即将执行。
	JobStarted(jobKey string)
	// JobCompleted 记录任务成功及其实际执行耗时。
	JobCompleted(jobKey string, duration time.Duration)
	// JobFailed 记录任务失败及其实际执行耗时。
	JobFailed(jobKey string, duration time.Duration)
	// JobSkipped 记录任务在指定准入阶段被跳过。
	JobSkipped(jobKey, reason string)
	// JobLockRenewFailed 记录任务持有锁期间发生续租失败。
	JobLockRenewFailed(jobKey string)
}

// NopMetrics 是默认空实现，调用方未接入监控时保持零副作用。
type NopMetrics struct{}

// JobTriggered 实现 Metrics，并保持零副作用。
func (NopMetrics) JobTriggered(string) {}

// JobStarted 实现 Metrics，并保持零副作用。
func (NopMetrics) JobStarted(string) {}

// JobCompleted 实现 Metrics，并保持零副作用。
func (NopMetrics) JobCompleted(string, time.Duration) {}

// JobFailed 实现 Metrics，并保持零副作用。
func (NopMetrics) JobFailed(string, time.Duration) {}

// JobSkipped 实现 Metrics，并保持零副作用。
func (NopMetrics) JobSkipped(string, string) {}

// JobLockRenewFailed 实现 Metrics，并保持零副作用。
func (NopMetrics) JobLockRenewFailed(string) {}
