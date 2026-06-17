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
)

var allowedLowCardinalityLabelKeys = map[string]struct{}{
	LabelService:     {},
	LabelEnvironment: {},
	LabelMethod:      {},
	LabelRoute:       {},
	LabelStatusClass: {},
	LabelCode:        {},
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
