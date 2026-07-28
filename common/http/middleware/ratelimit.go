package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/aegiscore/common/http/response"
	"github.com/aegiscore/common/security/auth"
)

const (
	defaultRateLimitShards  = 64
	defaultRateLimitMessage = "too many requests"
)

var (
	// ErrRateLimitKeyRequired 表示调用方未提供可用的限流身份 key。
	ErrRateLimitKeyRequired = errors.New("rate limit key is required")
	// ErrRateLimiterClosed 表示限流器已关闭。
	ErrRateLimiterClosed = errors.New("rate limiter is closed")
)

// RateLimiter 是 Gin 限流 middleware 消费的最小限流判定接口。
type RateLimiter interface {
	Allow(key string) (bool, error)
}

// RateLimitKeyFunc 从 Gin 请求上下文解析限流身份 key。
type RateLimitKeyFunc func(*gin.Context) string

// RateLimitErrorFunc 处理限流器错误，调用方可用于记录日志或 metrics。
type RateLimitErrorFunc func(c *gin.Context, key string, err error)

// RateLimitLimitedFunc 处理请求被限流事件，调用方可用于记录日志或 metrics。
type RateLimitLimitedFunc func(c *gin.Context, key string)

// RateLimitOptions 配置 Gin 限流 middleware。
type RateLimitOptions struct {
	Limiter    RateLimiter
	KeyFunc    RateLimitKeyFunc
	Message    string
	FailClosed bool
	OnError    RateLimitErrorFunc
	OnLimit    RateLimitLimitedFunc
}

// LocalRateLimiterOptions 配置本地 token bucket 限流器。
type LocalRateLimiterOptions struct {
	Rate            rate.Limit
	Burst           int
	Shards          int
	KeyTTL          time.Duration
	CleanupInterval time.Duration
	Now             func() time.Time
}

// LocalRateLimiter 是基于 golang.org/x/time/rate 的业务中立本地限流器。
type LocalRateLimiter struct {
	rate            rate.Limit
	burst           int
	keyTTL          time.Duration
	cleanupInterval time.Duration
	now             func() time.Time
	shards          []rateLimitShard
	ctx             context.Context
	cancel          context.CancelFunc
	cleanupCursor   int
	startOnce       sync.Once
	stopOnce        sync.Once
	closedMu        sync.RWMutex
	closed          bool
}

type rateLimitShard struct {
	mu       sync.Mutex
	visitors map[string]*rateLimitVisitor
}

type rateLimitVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewLocalRateLimiter 创建本地分片 token bucket 限流器。
func NewLocalRateLimiter(options LocalRateLimiterOptions) (*LocalRateLimiter, error) {
	if options.Rate <= 0 {
		return nil, fmt.Errorf("rate must be > 0")
	}
	if options.Burst <= 0 {
		return nil, fmt.Errorf("burst must be > 0")
	}
	if options.KeyTTL <= 0 {
		return nil, fmt.Errorf("key ttl must be > 0")
	}
	if options.CleanupInterval <= 0 {
		return nil, fmt.Errorf("cleanup interval must be > 0")
	}
	shardCount := options.Shards
	if shardCount <= 0 {
		shardCount = defaultRateLimitShards
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	limiter := &LocalRateLimiter{
		rate:            options.Rate,
		burst:           options.Burst,
		keyTTL:          options.KeyTTL,
		cleanupInterval: options.CleanupInterval,
		now:             now,
		shards:          make([]rateLimitShard, shardCount),
		ctx:             ctx,
		cancel:          cancel,
	}
	for i := range limiter.shards {
		limiter.shards[i].visitors = make(map[string]*rateLimitVisitor)
	}
	return limiter, nil
}

// Allow 判断 key 当前是否允许通过。
func (l *LocalRateLimiter) Allow(key string) (bool, error) {
	if l == nil || l.isClosed() {
		return false, ErrRateLimiterClosed
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return false, ErrRateLimitKeyRequired
	}
	shard := l.shardFor(key)
	now := l.now()

	shard.mu.Lock()
	visitor := shard.visitors[key]
	if visitor == nil {
		visitor = &rateLimitVisitor{limiter: rate.NewLimiter(l.rate, l.burst)}
		shard.visitors[key] = visitor
	}
	visitor.lastSeen = now
	allowed := visitor.limiter.AllowN(now, 1)
	shard.mu.Unlock()

	return allowed, nil
}

// StartJanitor 启动后台清理 goroutine。重复调用不会创建多个 janitor。
// 调用方需要在资源生命周期启动阶段显式调用 StartJanitor，并在停止阶段调用 Close；否则过期 key 只会在显式 Cleanup 时清理。
func (l *LocalRateLimiter) StartJanitor() {
	if l == nil || l.isClosed() {
		return
	}
	l.startOnce.Do(func() {
		go l.runJanitor()
	})
}

// Close 停止后台清理资源，不关闭任何调用方资源。
func (l *LocalRateLimiter) Close() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() {
		l.closedMu.Lock()
		l.closed = true
		l.closedMu.Unlock()
		l.cancel()
	})
}

func (l *LocalRateLimiter) runJanitor() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.ctx.Done():
			return
		case now := <-ticker.C:
			l.cleanupNextShards(now)
		}
	}
}

// Cleanup 删除超过 key TTL 未访问的 limiter。测试可直接调用该方法验证清理语义。
func (l *LocalRateLimiter) Cleanup(now time.Time) {
	if l == nil || l.isClosed() {
		return
	}
	for i := range l.shards {
		l.cleanupShard(i, now)
	}
}

func (l *LocalRateLimiter) cleanupNextShards(now time.Time) {
	if l == nil || l.isClosed() {
		return
	}
	limit := max(1, len(l.shards)/16)
	for range limit {
		index := l.cleanupCursor % len(l.shards)
		l.cleanupShard(index, now)
		l.cleanupCursor = (l.cleanupCursor + 1) % len(l.shards)
	}
}

func (l *LocalRateLimiter) cleanupShard(index int, now time.Time) {
	shard := &l.shards[index]
	shard.mu.Lock()
	for key, visitor := range shard.visitors {
		if now.Sub(visitor.lastSeen) > l.keyTTL {
			delete(shard.visitors, key)
		}
	}
	shard.mu.Unlock()
}

// Len 返回当前本地 limiter key 数量，仅用于测试和诊断。
func (l *LocalRateLimiter) Len() int {
	if l == nil {
		return 0
	}
	total := 0
	for i := range l.shards {
		shard := &l.shards[i]
		shard.mu.Lock()
		total += len(shard.visitors)
		shard.mu.Unlock()
	}
	return total
}

func (l *LocalRateLimiter) isClosed() bool {
	l.closedMu.RLock()
	defer l.closedMu.RUnlock()
	return l.closed
}

func (l *LocalRateLimiter) shardFor(key string) *rateLimitShard {
	const (
		fnvOffset32 = 2166136261
		fnvPrime32  = 16777619
	)
	hash := uint32(fnvOffset32)
	for i := range len(key) {
		hash ^= uint32(key[i])
		hash *= fnvPrime32
	}
	return &l.shards[int(hash)%len(l.shards)]
}

// RateLimit 返回 Gin 请求限流 middleware。
func RateLimit(options RateLimitOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		if options.Limiter == nil {
			c.Next()
			return
		}
		key := ""
		if options.KeyFunc != nil {
			key = strings.TrimSpace(options.KeyFunc(c))
		}
		allowed, err := options.Limiter.Allow(key)
		if err != nil {
			if options.OnError != nil {
				options.OnError(c, key, err)
			}
			// 限流默认不是认证或授权边界；key 缺失、关闭或内部错误时默认降级放行，由后续 middleware 保持原有语义。
			if options.FailClosed {
				message := strings.TrimSpace(options.Message)
				if message == "" {
					message = defaultRateLimitMessage
				}
				response.RateLimited(c, message)
				c.Abort()
				return
			}
			c.Next()
			return
		}
		if !allowed {
			if options.OnLimit != nil {
				options.OnLimit(c, key)
			}
			message := strings.TrimSpace(options.Message)
			if message == "" {
				message = defaultRateLimitMessage
			}
			response.RateLimited(c, message)
			c.Abort()
			return
		}
		c.Next()
	}
}

// IPRateLimitKey 使用 Gin 解析出的客户端 IP 构造限流 key。
func IPRateLimitKey(prefix string) RateLimitKeyFunc {
	prefix = strings.TrimSpace(prefix)
	return func(c *gin.Context) string {
		ip := strings.TrimSpace(c.ClientIP())
		if ip == "" {
			return ""
		}
		return prefix + ":ip:" + ip
	}
}

// UserIDRateLimitKey 使用已认证请求上下文中的 User ID 构造限流 key。
func UserIDRateLimitKey(prefix string) RateLimitKeyFunc {
	prefix = strings.TrimSpace(prefix)
	return func(c *gin.Context) string {
		if userID, ok := auth.UserIDFromContext(c.Request.Context()); ok {
			userID = strings.TrimSpace(userID)
			if userID != "" {
				return prefix + ":user:" + userID
			}
		}
		value, ok := c.Get(auth.UserIDKey)
		if !ok {
			return ""
		}
		userID, ok := value.(string)
		if !ok || strings.TrimSpace(userID) == "" {
			return ""
		}
		return prefix + ":user:" + strings.TrimSpace(userID)
	}
}
