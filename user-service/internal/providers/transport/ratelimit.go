package transport

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"golang.org/x/time/rate"

	commonmw "github.com/aegiscore/common/http/middleware"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// APIRateLimiters 包含 user-service 路由使用的限流器。
type APIRateLimiters struct {
	Anonymous     commonmw.RateLimiter
	Authenticated commonmw.RateLimiter
	closers       []*commonmw.LocalRateLimiter
}

// NewAPIRateLimiters 根据服务私有限流配置构造本地限流资源并注册 lifecycle。
func NewAPIRateLimiters(lifecycle fx.Lifecycle, settings serviceconfig.RateLimitSettings) (*APIRateLimiters, error) {
	limiters := &APIRateLimiters{}
	if settings.APIRateLimit.Anonymous.Enabled {
		limiter, err := newLocalRateLimiter(settings.APIRateLimit.Anonymous)
		if err != nil {
			return nil, fmt.Errorf("anonymous rate limiter: %w", err)
		}
		limiters.Anonymous = limiter
		limiters.closers = append(limiters.closers, limiter)
	}
	if settings.APIRateLimit.Authenticated.Enabled {
		limiter, err := newLocalRateLimiter(settings.APIRateLimit.Authenticated)
		if err != nil {
			limiters.Close()
			return nil, fmt.Errorf("authenticated rate limiter: %w", err)
		}
		limiters.Authenticated = limiter
		limiters.closers = append(limiters.closers, limiter)
	}
	if len(limiters.closers) > 0 {
		lifecycle.Append(fx.Hook{
			OnStart: func(context.Context) error {
				for _, limiter := range limiters.closers {
					limiter.StartJanitor()
				}
				return nil
			},
			OnStop: func(context.Context) error {
				limiters.Close()
				return nil
			},
		})
	}
	return limiters, nil
}

// Close 停止限流器自有后台资源。
func (l *APIRateLimiters) Close() {
	if l == nil {
		return
	}
	for _, limiter := range l.closers {
		limiter.Close()
	}
}

func newLocalRateLimiter(cfg serviceconfig.RateLimitPolicyConfig) (*commonmw.LocalRateLimiter, error) {
	return commonmw.NewLocalRateLimiter(commonmw.LocalRateLimiterOptions{
		Rate:            rate.Limit(cfg.RatePerSecond),
		Burst:           cfg.Burst,
		Shards:          cfg.Shards,
		MaxKeys:         cfg.MaxKeys,
		CapacityPolicy:  commonmw.RateLimitCapacityPolicy(cfg.CapacityPolicy),
		KeyTTL:          cfg.KeyTTL,
		CleanupInterval: cfg.CleanupInterval,
	})
}
