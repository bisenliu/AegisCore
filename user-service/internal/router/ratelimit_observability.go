package router

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	commonmw "github.com/aegiscore/common/http/middleware"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
)

const (
	// apiRateLimitEventsMetricName 是接口限流触发事件统计 Counter 指标名。
	apiRateLimitEventsMetricName = "aegiscore_user_service_api_rate_limit_events_total"
	// apiRateLimitEventsMetricHelp 是接口限流事件指标说明。
	apiRateLimitEventsMetricHelp = "Total number of API rate limit events by fixed scope, event, and reason."

	// rateLimitEventError 表示限流检查过程发生内部错误事件。
	rateLimitEventError = "error"
	// rateLimitEventLimited 表示请求触发配额上限被成功拦截限流事件。
	rateLimitEventLimited = "limited"

	// rateLimitReasonLimitExceeded 表示访问频次超过设定窗口配额。
	rateLimitReasonLimitExceeded = "limit_exceeded"
	// rateLimitReasonKeyRequired 表示请求未提供识别身份的限流 key。
	rateLimitReasonKeyRequired = "key_required"
	// rateLimitReasonLimiterClosed 表示限流器组件已关闭停止接受请求。
	rateLimitReasonLimiterClosed = "limiter_closed"
	// rateLimitReasonOverflow 表示独立限流名额已满进入公共备用配额池。
	rateLimitReasonOverflow = "overflow"
	// rateLimitReasonRejected 表示限流名单已满拒绝服务。
	rateLimitReasonRejected = "rejected"
	// rateLimitReasonError 表示因底层错误导致的异常。
	rateLimitReasonError = "error"
)

type rateLimitObserver struct {
	log    *zap.Logger
	events *prometheus.CounterVec
}

// newRateLimitObserver 创建记录频次超限日志和报警指标的监控组件。
func newRateLimitObserver(log *zap.Logger, provider *commonmetrics.Provider) (*rateLimitObserver, error) {
	if log == nil {
		log = zap.NewNop()
	}
	observer := &rateLimitObserver{log: log}
	if provider == nil || !provider.Enabled() {
		return observer, nil
	}
	observer.events = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: apiRateLimitEventsMetricName,
		Help: apiRateLimitEventsMetricHelp,
	}, []string{"scope", commonmetrics.LabelEvent, commonmetrics.LabelReason})
	if err := provider.Register(observer.events); err != nil {
		return nil, err
	}
	return observer, nil
}

// options 组装不同业务接口的访问频率限制规则（如登录接口限制更严，查询接口相对宽松），并挂接超限拦截记录。
func (o *rateLimitObserver) options(scope string, limiter commonmw.RateLimiter, keyFunc commonmw.RateLimitKeyFunc) commonmw.RateLimitOptions {
	return commonmw.RateLimitOptions{
		Limiter: limiter,
		KeyFunc: keyFunc,
		OnError: func(c *gin.Context, key string, err error) {
			o.recordError(c, scope, key, err)
		},
		OnLimit: func(c *gin.Context, key string) {
			o.recordLimited(c, scope, key)
		},
	}
}

// recordError 记录频次限制判断过程中发生的异常，杜绝在日志中打印用户的真实输入内容。
func (o *rateLimitObserver) recordError(_ *gin.Context, scope string, key string, err error) {
	if o == nil {
		return
	}
	reason := rateLimitErrorReason(err)
	if o.events != nil {
		o.events.WithLabelValues(scope, rateLimitEventError, reason).Inc()
	}
	o.log.Warn("api rate limiter failed", zap.String("scope", scope), zap.String("reason", reason), zap.Bool("key_present", key != ""), zap.Error(err))
}

// recordLimited 记录某次用户访问因频次超限而被系统成功拦截的事件。
func (o *rateLimitObserver) recordLimited(_ *gin.Context, scope string, key string) {
	if o == nil {
		return
	}
	if o.events != nil {
		o.events.WithLabelValues(scope, rateLimitEventLimited, rateLimitReasonLimitExceeded).Inc()
	}
	o.log.Debug("api request rate limited", zap.String("scope", scope), zap.Bool("key_present", key != ""))
}

// rateLimitErrorReason 将频次检查失败的原因归纳为统一标识。
func rateLimitErrorReason(err error) string {
	switch {
	case errors.Is(err, commonmw.ErrRateLimitKeyRequired):
		return rateLimitReasonKeyRequired
	case errors.Is(err, commonmw.ErrRateLimiterClosed):
		return rateLimitReasonLimiterClosed
	case errors.Is(err, commonmw.ErrRateLimitCapacityOverflow):
		return rateLimitReasonOverflow
	case errors.Is(err, commonmw.ErrRateLimitCapacityRejected):
		return rateLimitReasonRejected
	default:
		return rateLimitReasonError
	}
}
