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
	// LabelResource 标识固定运行时资源名，例如 user_db 或 cache_redis。
	LabelResource = "resource"
	// LabelPool 标识固定后台任务池名称。
	LabelPool = "pool"
	// LabelJob 标识固定定时任务名称。
	LabelJob = "job"
	// LabelEvent 标识固定运行时事件名称。
	LabelEvent = "event"
	// LabelStatus 标识固定运行时状态。
	LabelStatus = "status"
	// LabelReason 标识固定跳过或失败原因枚举。
	LabelReason = "reason"
)

var allowedLowCardinalityLabelKeys = map[string]struct{}{
	LabelService:     {},
	LabelEnvironment: {},
	LabelMethod:      {},
	LabelRoute:       {},
	LabelStatusClass: {},
	LabelCode:        {},
	LabelResource:    {},
	LabelPool:        {},
	LabelJob:         {},
	LabelEvent:       {},
	LabelStatus:      {},
	LabelReason:      {},
}

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
	if _, ok := allowedLowCardinalityLabelKeys[key]; ok {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrUnsupportedLabelKey, key)
}
