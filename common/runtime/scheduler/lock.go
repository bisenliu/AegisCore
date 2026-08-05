package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"

	"github.com/aegiscore/common/runtime/rediskey"
)

// Locker 定义调度器使用的分布式锁能力。
type Locker interface {
	// Acquire 在 waitTimeout 总时限内获取指定 key 的 owner lock。
	Acquire(ctx context.Context, key string, ttl time.Duration, waitTimeout time.Duration) (Lock, bool, error)
}

// Lock 定义已持有锁的释放和续租能力。
type Lock interface {
	// Unlock 仅允许当前 owner 释放锁。
	Unlock(ctx context.Context) error
	// Renew 仅允许当前 owner 刷新锁 TTL。
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

type redisLock struct {
	client redis.UniversalClient
	key    string
	token  string
}

var (
	// Script 会缓存 SHA 并在 NOSCRIPT 时回退 EVAL，所有调用仍通过 owner token 校验。
	redisUnlockScript = redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`)
	redisRenewScript  = redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) else return 0 end`)
)

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
// 返回 nil,false,nil 表示锁被其他 owner 持有且本次未取得，不是系统错误；MaxAttempts 和 deadline 会共同限制重试。
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
	if waitTimeout < 0 {
		return nil, false, fmt.Errorf("%w: wait timeout must not be negative", ErrInvalidLock)
	}

	fullKey, err := l.builder.Key(key)
	if err != nil {
		return nil, false, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, false, err
	}

	if waitTimeout == 0 {
		acquired, err := l.client.SetNX(ctx, fullKey, token, ttl).Result()
		if err != nil {
			return nil, false, err
		}
		if !acquired {
			return nil, false, nil
		}
		return &redisLock{client: l.client, key: fullKey, token: token}, true, nil
	}

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	// waitCtx 表达所有重试共享的总等待上限，单次退避不会越过该 deadline。
	attempt := 1
	nextDelay := l.retry.InitialInterval
	for {
		acquired, err := l.client.SetNX(waitCtx, fullKey, token, ttl).Result()
		if err != nil {
			if ctx.Err() != nil {
				return nil, false, ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) || waitCtx.Err() != nil {
				return nil, false, nil
			}
			return nil, false, err
		}
		if acquired {
			return &redisLock{client: l.client, key: fullKey, token: token}, true, nil
		}
		if l.retry.MaxAttempts > 0 && attempt >= l.retry.MaxAttempts {
			return nil, false, nil
		}

		sleep := l.retryDelay(nextDelay)
		deadline, _ := waitCtx.Deadline()
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
			stopTimer(timer)
			return nil, false, ctx.Err()
		case <-waitCtx.Done():
			stopTimer(timer)
			if ctx.Err() != nil {
				return nil, false, ctx.Err()
			}
			return nil, false, nil
		case <-timer.C:
		}
		attempt++
		nextDelay = nextRetryDelay(nextDelay, l.retry.MaxInterval)
	}
}

// Unlock 仅在当前 token 仍持有锁时释放 Redis key。
func (l *redisLock) Unlock(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := redisUnlockScript.Run(ctx, l.client, []string{l.key}, l.token).Int()
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
	result, err := redisRenewScript.Run(ctx, l.client, []string{l.key}, l.token, ttl.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrLockNotOwned
	}
	return nil
}

// randomToken 生成单次 acquire 使用的随机 owner token。
func randomToken() (string, error) {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

// normalizeRetryPolicy 填充 retry 默认值并拒绝无法执行的区间或次数配置。
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

// retryDelay 根据配置返回当前退避时长，并在启用时施加有界 jitter。
func (l *RedisLocker) retryDelay(delay time.Duration) time.Duration {
	// jitter 在 [delay/2, delay] 内随机；随机源异常时回退原 delay，避免锁竞争路径引入额外错误。
	if !l.retry.Jitter || delay <= 1 {
		return delay
	}
	min := delay / 2
	if min <= 0 {
		min = time.Millisecond
	}
	spread := delay - min
	offset, err := rand.Int(rand.Reader, big.NewInt(int64(spread)+1))
	if err != nil {
		return delay
	}
	return min + time.Duration(offset.Int64())
}

// nextRetryDelay 计算下一次指数退避，并在溢出或超过上限时截断。
func nextRetryDelay(current time.Duration, max time.Duration) time.Duration {
	// 倍增退避在溢出或超过上限时截断到 max。
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

// stopTimer 在取消分支回收 timer，且兼容 timer channel 已触发或已被消费的状态。
func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}
