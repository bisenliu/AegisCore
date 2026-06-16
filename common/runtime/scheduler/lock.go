package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand/v2"
	"strings"
	"time"

	"github.com/aegiscore/common/runtime/rediskey"
	redis "github.com/redis/go-redis/v9"
)

// Locker 定义调度器使用的分布式锁能力。
type Locker interface {
	Acquire(ctx context.Context, key string, ttl time.Duration, waitTimeout time.Duration) (Lock, bool, error)
}

// Lock 定义已持有锁的释放和续租能力。
type Lock interface {
	Unlock(ctx context.Context) error
	Renew(ctx context.Context, ttl time.Duration) error
}

// RedisLockerOptions 配置 Redis 分布式锁实现。
type RedisLockerOptions struct {
	Namespace string
	Scope     []string
	Retry     RetryPolicy
}

// RetryPolicy 配置锁竞争时的重试退避策略。
type RetryPolicy struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	MaxAttempts     int
	Jitter          bool
}

// RedisLocker 基于 Redis SET NX PX 和 Lua owner 校验实现分布式锁。
type RedisLocker struct {
	client  redis.UniversalClient
	builder rediskey.Builder
	retry   RetryPolicy
}

// NewRedisLocker 使用已有 Redis client 构造分布式锁实现。
func NewRedisLocker(client redis.UniversalClient, opts RedisLockerOptions) (*RedisLocker, error) {
	if client == nil {
		return nil, fmt.Errorf("%w: redis client is required", ErrInvalidLock)
	}
	builder, err := rediskey.NewBuilder(rediskey.Options{Namespace: opts.Namespace})
	if err != nil {
		return nil, err
	}
	if len(opts.Scope) > 0 {
		builder, err = builder.Scoped(opts.Scope...)
		if err != nil {
			return nil, err
		}
	}

	retry, err := normalizeRetryPolicy(opts.Retry)
	if err != nil {
		return nil, err
	}

	return &RedisLocker{
		client:  client,
		builder: builder,
		retry:   retry,
	}, nil
}

// Acquire 尝试获取 Redis 锁；waitTimeout 为 0 时只尝试一次。
func (l *RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration, waitTimeout time.Duration) (Lock, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, fmt.Errorf("%w: key is required", ErrInvalidLock)
	}
	if ttl <= 0 {
		return nil, false, fmt.Errorf("%w: ttl must be positive", ErrInvalidLock)
	}

	fullKey, err := l.builder.Key(key)
	if err != nil {
		return nil, false, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, false, err
	}

	deadline := time.Now().Add(waitTimeout)
	attempt := 1
	nextDelay := l.retry.InitialInterval
	for {
		acquired, err := l.client.SetNX(ctx, fullKey, token, ttl).Result()
		if err != nil {
			return nil, false, err
		}
		if acquired {
			return &redisLock{client: l.client, key: fullKey, token: token}, true, nil
		}
		if waitTimeout <= 0 || time.Now().After(deadline) {
			return nil, false, nil
		}
		if l.retry.MaxAttempts > 0 && attempt >= l.retry.MaxAttempts {
			return nil, false, nil
		}

		sleep := l.retryDelay(nextDelay)
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, false, nil
		}
		if sleep > remaining {
			sleep = remaining
		}

		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, false, ctx.Err()
		case <-timer.C:
		}
		attempt++
		nextDelay = nextRetryDelay(nextDelay, l.retry.MaxInterval)
	}
}

type redisLock struct {
	client redis.UniversalClient
	key    string
	token  string
}

// Unlock 仅在当前 token 仍持有锁时释放 Redis key。
func (l *redisLock) Unlock(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	const script = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`
	result, err := l.client.Eval(ctx, script, []string{l.key}, l.token).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrLockNotOwned
	}
	return nil
}

// Renew 仅在当前 token 仍持有锁时刷新 Redis key TTL。
func (l *redisLock) Renew(ctx context.Context, ttl time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ttl <= 0 {
		return fmt.Errorf("%w: ttl must be positive", ErrInvalidLock)
	}
	const script = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) else return 0 end`
	result, err := l.client.Eval(ctx, script, []string{l.key}, l.token, ttl.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrLockNotOwned
	}
	return nil
}

func randomToken() (string, error) {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

func normalizeRetryPolicy(policy RetryPolicy) (RetryPolicy, error) {
	if policy.InitialInterval <= 0 {
		policy.InitialInterval = 50 * time.Millisecond
	}
	if policy.MaxInterval <= 0 {
		policy.MaxInterval = time.Second
	}
	if policy.InitialInterval > policy.MaxInterval {
		return RetryPolicy{}, fmt.Errorf("%w: retry initial interval must not be greater than max interval", ErrInvalidLock)
	}
	if policy.MaxAttempts < 0 {
		return RetryPolicy{}, fmt.Errorf("%w: retry max attempts must not be negative", ErrInvalidLock)
	}
	return policy, nil
}

func (l *RedisLocker) retryDelay(delay time.Duration) time.Duration {
	if !l.retry.Jitter || delay <= 1 {
		return delay
	}
	min := delay / 2
	if min <= 0 {
		min = time.Millisecond
	}
	spread := delay - min
	return min + time.Duration(mathrand.Int64N(int64(spread)+1))
}

func nextRetryDelay(current time.Duration, max time.Duration) time.Duration {
	if current <= 0 {
		return max
	}
	if current >= max {
		return max
	}
	next := current * 2
	if next < current || next > max {
		return max
	}
	return next
}
