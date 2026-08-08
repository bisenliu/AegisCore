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
	apiRateLimitEventsMetricName = "aegiscore_user_service_api_rate_limit_events_total"
	apiRateLimitEventsMetricHelp = "Total number of API rate limit events by fixed scope, event, and reason."

	rateLimitEventError   = "error"
	rateLimitEventLimited = "limited"

	rateLimitReasonLimitExceeded = "limit_exceeded"
	rateLimitReasonKeyRequired   = "key_required"
	rateLimitReasonLimiterClosed = "limiter_closed"
	rateLimitReasonOverflow      = "overflow"
	rateLimitReasonRejected      = "rejected"
	rateLimitReasonError         = "error"
)

type rateLimitObserver struct {
	log    *zap.Logger
	events *prometheus.CounterVec
}

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

func (o *rateLimitObserver) recordLimited(_ *gin.Context, scope string, key string) {
	if o == nil {
		return
	}
	if o.events != nil {
		o.events.WithLabelValues(scope, rateLimitEventLimited, rateLimitReasonLimitExceeded).Inc()
	}
	o.log.Debug("api request rate limited", zap.String("scope", scope), zap.Bool("key_present", key != ""))
}

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
