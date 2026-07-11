package metrics

import (
	"errors"
	"fmt"
)

const (
	// LabelService 标识产生指标的服务名，由 provider 作为 const label 注入。
	LabelService = "service"
	// LabelEnvironment 标识部署环境，由 provider 作为 const label 注入。
	LabelEnvironment = "environment"
	// LabelMethod 标识 HTTP method 或有限集合的 runtime action。
	LabelMethod = "method"
	// LabelRoute 标识 HTTP route template 或固定 runtime key。
	LabelRoute = "route"
	// LabelStatusClass 标识 HTTP status class，例如 2xx。
	LabelStatusClass = "status_class"
	// LabelCode 标识稳定错误码或结果码，不能使用原始错误字符串。
	LabelCode = "code"
	// LabelResult 标识固定结果枚举，例如 hit、miss、success 或 error。
	LabelResult = "result"
	// LabelResource 标识固定运行时资源名，例如 user_db 或 cache_redis。
	LabelResource = "resource"
	// LabelCache 标识固定本地缓存实例名。
	LabelCache = "cache"
	// LabelPool 标识固定后台任务池名称。
	LabelPool = "pool"
	// LabelSchedulerJob 标识固定定时任务名称，避免与 Prometheus scrape job label 冲突。
	LabelSchedulerJob = "scheduler_job"
	// LabelEvent 标识固定运行时事件名称。
	LabelEvent = "event"
	// LabelStatus 标识固定运行时状态。
	LabelStatus = "status"
	// LabelReason 标识固定跳过或失败原因枚举。
	LabelReason = "reason"
)

// ErrUnsupportedLabelKey 表示 label key 不属于当前跨服务低基数约定。
var ErrUnsupportedLabelKey = errors.New("unsupported metrics label key")

// StatusClass 将 HTTP status code 归一化为低基数 status class。
func StatusClass(status int) string {
	if status < 100 || status > 599 {
		return "unknown"
	}
	return fmt.Sprintf("%dxx", status/100)
}

// ValidateLowCardinalityLabelKey 校验通用 metrics helper 允许的低基数 label key。
func ValidateLowCardinalityLabelKey(key string) error {
	switch key {
	case LabelService, LabelEnvironment, LabelMethod, LabelRoute, LabelStatusClass, LabelCode,
		LabelResult, LabelResource, LabelCache, LabelPool, LabelSchedulerJob, LabelEvent, LabelStatus, LabelReason:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedLabelKey, key)
	}
}
