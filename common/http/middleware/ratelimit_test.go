package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/security/auth"
)

func TestLocalRateLimiterAllowAndCleanup(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(1), Burst: 1, Shards: 4, MaxKeys: 8, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour, Now: func() time.Time { return now }})

	allowed, err := limiter.Allow("user:1")
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = limiter.Allow("user:1")
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 1, limiter.Len())

	limiter.Cleanup(now.Add(2 * time.Minute))
	require.Equal(t, 0, limiter.Len())
}

func TestLocalRateLimiterRejectsMissingKeyAndClosedLimiter(t *testing.T) {
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(1), Burst: 1, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})

	allowed, err := limiter.Allow(" ")
	require.ErrorIs(t, err, ErrRateLimitKeyRequired)
	require.False(t, allowed)

	limiter.Close()
	allowed, err = limiter.Allow("user:1")
	require.ErrorIs(t, err, ErrRateLimiterClosed)
	require.False(t, allowed)
}

func TestLocalRateLimiterSupportsConcurrentKeys(t *testing.T) {
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(1000), Burst: 1000, Shards: 8, MaxKeys: 100, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			allowed, err := limiter.Allow(string(rune('a' + i%26)))
			require.NoError(t, err)
			require.True(t, allowed)
		}(i)
	}
	wg.Wait()
	require.Equal(t, 26, limiter.Len())
}

func TestLocalRateLimiterBoundsUniqueKeysWithOverflow(t *testing.T) {
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(100), Burst: 2, Shards: 1, MaxKeys: 2, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})

	for _, key := range []string{"ip:1", "ip:2"} {
		allowed, err := limiter.Allow(key)
		require.NoError(t, err)
		require.True(t, allowed)
	}
	require.Equal(t, 2, limiter.Len())

	for _, key := range []string{"ip:3", "ip:4"} {
		allowed, err := limiter.Allow(key)
		require.ErrorIs(t, err, ErrRateLimitCapacityOverflow)
		require.True(t, allowed)
		require.Equal(t, 2, limiter.Len())
	}

	allowed, err := limiter.Allow("ip:5")
	require.ErrorIs(t, err, ErrRateLimitCapacityOverflow)
	require.False(t, allowed)
	require.Equal(t, 2, limiter.Len())
}

func TestLocalRateLimiterUsesGlobalCapacityAcrossShards(t *testing.T) {
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(100), Burst: 100, Shards: 8, MaxKeys: 2, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})

	for _, key := range []string{"ip:1", "ip:2"} {
		allowed, err := limiter.Allow(key)
		require.NoError(t, err)
		require.True(t, allowed)
	}
	allowed, err := limiter.Allow("ip:3")
	require.ErrorIs(t, err, ErrRateLimitCapacityOverflow)
	require.True(t, allowed)
	require.Equal(t, 2, limiter.Len())
}

func TestLocalRateLimiterRejectsNewKeysWhenCapacityFull(t *testing.T) {
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(100), Burst: 100, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyReject, KeyTTL: time.Minute, CleanupInterval: time.Hour})

	allowed, err := limiter.Allow("ip:1")
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = limiter.Allow("ip:2")
	require.ErrorIs(t, err, ErrRateLimitCapacityRejected)
	require.False(t, allowed)
	require.Equal(t, 1, limiter.Len())
}

func TestLocalRateLimiterPreservesExistingKeyStateWhenCapacityFull(t *testing.T) {
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(1), Burst: 1, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})

	allowed, err := limiter.Allow("ip:1")
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = limiter.Allow("ip:2")
	require.ErrorIs(t, err, ErrRateLimitCapacityOverflow)
	require.True(t, allowed)
	allowed, err = limiter.Allow("ip:1")
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 1, limiter.Len())
}

func TestLocalRateLimiterCleanupReleasesCapacity(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(100), Burst: 100, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyReject, KeyTTL: time.Minute, CleanupInterval: time.Hour, Now: func() time.Time { return now }})

	allowed, err := limiter.Allow("ip:1")
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = limiter.Allow("ip:2")
	require.ErrorIs(t, err, ErrRateLimitCapacityRejected)
	require.False(t, allowed)

	limiter.Cleanup(now.Add(2 * time.Minute))
	require.Equal(t, 0, limiter.Len())
	allowed, err = limiter.Allow("ip:2")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, 1, limiter.Len())
}

func TestRateLimitMiddlewareWritesRateLimitedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(1), Burst: 1, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})
	engine := gin.New()
	engine.Use(RateLimit(RateLimitOptions{Limiter: limiter, KeyFunc: func(*gin.Context) string { return "ip:203.0.113.1" }, Message: "请求过于频繁"}))
	engine.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	first := httptest.NewRecorder()
	engine.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/limited", nil))
	require.Equal(t, http.StatusNoContent, first.Code)

	second := httptest.NewRecorder()
	engine.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/limited", nil))
	require.Equal(t, http.StatusTooManyRequests, second.Code)
	var envelope contractresponse.Envelope
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &envelope))
	require.False(t, envelope.Success)
	require.Equal(t, contracterrors.CodeRateLimited, envelope.Code)
	require.Equal(t, "请求过于频繁", envelope.Message)
}

func TestRateLimitMiddlewarePassesThroughMissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(1), Burst: 1, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})
	engine := gin.New()
	var gotKey string
	var gotErr error
	engine.Use(RateLimit(RateLimitOptions{Limiter: limiter, OnError: func(_ *gin.Context, key string, err error) {
		gotKey = key
		gotErr = err
	}}))
	engine.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/limited", nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Empty(t, gotKey)
	require.ErrorIs(t, gotErr, ErrRateLimitKeyRequired)
}

func TestRateLimitMiddlewarePassesThroughClosedLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(1), Burst: 1, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})
	limiter.Close()
	engine := gin.New()
	engine.Use(RateLimit(RateLimitOptions{Limiter: limiter, KeyFunc: func(*gin.Context) string { return "user:1" }}))
	engine.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/limited", nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestRateLimitMiddlewareFailClosedWritesRateLimitedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(1), Burst: 1, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})
	engine := gin.New()
	var gotErr error
	engine.Use(RateLimit(RateLimitOptions{Limiter: limiter, FailClosed: true, Message: "限流器不可用", OnError: func(_ *gin.Context, _ string, err error) {
		gotErr = err
	}}))
	engine.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/limited", nil))
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.ErrorIs(t, gotErr, ErrRateLimitKeyRequired)
	var envelope contractresponse.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.False(t, envelope.Success)
	require.Equal(t, contracterrors.CodeRateLimited, envelope.Code)
	require.Equal(t, "限流器不可用", envelope.Message)
}

func TestRateLimitMiddlewareCallsOnLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(1), Burst: 1, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})
	engine := gin.New()
	var limitedKey string
	engine.Use(RateLimit(RateLimitOptions{Limiter: limiter, KeyFunc: func(*gin.Context) string { return "ip:203.0.113.1" }, OnLimit: func(_ *gin.Context, key string) {
		limitedKey = key
	}}))
	engine.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	first := httptest.NewRecorder()
	engine.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/limited", nil))
	require.Equal(t, http.StatusNoContent, first.Code)
	require.Empty(t, limitedKey)

	second := httptest.NewRecorder()
	engine.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/limited", nil))
	require.Equal(t, http.StatusTooManyRequests, second.Code)
	require.Equal(t, "ip:203.0.113.1", limitedKey)
}

func TestRateLimitMiddlewareRecordsCapacityOverflowAndAllowsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(100), Burst: 100, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyOverflow, KeyTTL: time.Minute, CleanupInterval: time.Hour})
	allowed, err := limiter.Allow("ip:existing")
	require.NoError(t, err)
	require.True(t, allowed)
	engine := gin.New()
	var gotErr error
	engine.Use(RateLimit(RateLimitOptions{Limiter: limiter, KeyFunc: func(*gin.Context) string { return "ip:new" }, OnError: func(_ *gin.Context, _ string, err error) {
		gotErr = err
	}}))
	engine.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/limited", nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.ErrorIs(t, gotErr, ErrRateLimitCapacityOverflow)
}

func TestRateLimitMiddlewareRejectsCapacityRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newTestLocalRateLimiter(t, LocalRateLimiterOptions{Rate: rate.Limit(100), Burst: 100, Shards: 1, MaxKeys: 1, CapacityPolicy: RateLimitCapacityPolicyReject, KeyTTL: time.Minute, CleanupInterval: time.Hour})
	allowed, err := limiter.Allow("ip:existing")
	require.NoError(t, err)
	require.True(t, allowed)
	engine := gin.New()
	var gotErr error
	var limitedKey string
	engine.Use(RateLimit(RateLimitOptions{Limiter: limiter, KeyFunc: func(*gin.Context) string { return "ip:new" }, OnError: func(_ *gin.Context, _ string, err error) {
		gotErr = err
	}, OnLimit: func(_ *gin.Context, key string) {
		limitedKey = key
	}}))
	engine.GET("/limited", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/limited", nil))
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.ErrorIs(t, gotErr, ErrRateLimitCapacityRejected)
	require.Equal(t, "ip:new", limitedKey)
}

func TestRateLimitKeyResolvers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx.Request.RemoteAddr = "203.0.113.10:12345"
	require.Equal(t, "anon:ip:203.0.113.10", IPRateLimitKey("anon")(ctx))

	ctx.Set(auth.UserIDKey, "user-1")
	require.Equal(t, "api:user:user-1", UserIDRateLimitKey("api")(ctx))

	ctx.Request = ctx.Request.WithContext(auth.WithUserID(ctx.Request.Context(), "user-from-context"))
	require.Equal(t, "api:user:user-from-context", UserIDRateLimitKey("api")(ctx))
	ctx.Request = ctx.Request.WithContext(auth.WithUserID(ctx.Request.Context(), ""))
	require.Equal(t, "api:user:user-1", UserIDRateLimitKey("api")(ctx))
}

func newTestLocalRateLimiter(t *testing.T, options LocalRateLimiterOptions) *LocalRateLimiter {
	t.Helper()
	limiter, err := NewLocalRateLimiter(options)
	require.NoError(t, err)
	t.Cleanup(limiter.Close)
	return limiter
}
