package transport

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	commonmw "github.com/aegiscore/common/http/middleware"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestNewAPIRateLimitersConstructsEnabledLimiters(t *testing.T) {
	lifecycle := fxtest.NewLifecycle(t)
	limiters, err := NewAPIRateLimiters(lifecycle, serviceconfig.RateLimitSettings{APIRateLimit: serviceconfig.APIRateLimitConfig{
		Anonymous:     serviceconfig.DefaultRateLimitPolicyConfig(1, 1, time.Minute, time.Hour, 2),
		Authenticated: serviceconfig.DefaultRateLimitPolicyConfig(2, 2, time.Minute, time.Hour, 2),
	}})
	require.NoError(t, err)
	require.NotNil(t, limiters.Anonymous)
	require.NotNil(t, limiters.Authenticated)

	lifecycle.RequireStart()
	lifecycle.RequireStop()

	allowed, err := limiters.Anonymous.Allow("ip:203.0.113.1")
	require.ErrorIs(t, err, commonmw.ErrRateLimiterClosed)
	require.False(t, allowed)
}

func TestNewAPIRateLimitersHonorsDisabledPolicies(t *testing.T) {
	lifecycle := fxtest.NewLifecycle(t)
	limiters, err := NewAPIRateLimiters(lifecycle, serviceconfig.RateLimitSettings{APIRateLimit: serviceconfig.APIRateLimitConfig{
		Anonymous:     serviceconfig.RateLimitPolicyConfig{Enabled: false},
		Authenticated: serviceconfig.RateLimitPolicyConfig{Enabled: false},
	}})
	require.NoError(t, err)
	require.Nil(t, limiters.Anonymous)
	require.Nil(t, limiters.Authenticated)
	require.NoError(t, lifecycle.Start(context.Background()))
	require.NoError(t, lifecycle.Stop(context.Background()))
}

func TestNewAPIRateLimitersRejectsInvalidEnabledPolicy(t *testing.T) {
	_, err := NewAPIRateLimiters(fxtest.NewLifecycle(t), serviceconfig.RateLimitSettings{APIRateLimit: serviceconfig.APIRateLimitConfig{
		Anonymous: serviceconfig.RateLimitPolicyConfig{Enabled: true},
	}})
	require.ErrorContains(t, err, "anonymous rate limiter")
}
