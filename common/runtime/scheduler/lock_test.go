package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
)

func TestRedisLockerAcquireUnlockAndReacquire(t *testing.T) {
	server, client := newMiniRedisClient(t)
	locker := newTestRedisLocker(t, client)

	lock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !ok {
		t.Fatal("Acquire ok = false, want true")
	}
	if !server.Exists("aegiscore:cron:nightly") {
		t.Fatal("lock key was not created")
	}

	secondLock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	if ok || secondLock != nil {
		t.Fatalf("second Acquire = (%v, %v), want busy", secondLock, ok)
	}

	if err := lock.Unlock(context.Background()); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if server.Exists("aegiscore:cron:nightly") {
		t.Fatal("lock key still exists after unlock")
	}

	_, ok, err = locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	if !ok {
		t.Fatal("reacquire ok = false, want true")
	}
}

func TestRedisLockUnlockRejectsLostOwnership(t *testing.T) {
	_, client := newMiniRedisClient(t)
	locker := newTestRedisLocker(t, client)

	lock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !ok {
		t.Fatal("Acquire ok = false, want true")
	}
	if err := client.Set(context.Background(), "aegiscore:cron:nightly", "other-owner", time.Minute).Err(); err != nil {
		t.Fatalf("Set other owner: %v", err)
	}

	err = lock.Unlock(context.Background())
	if !errors.Is(err, ErrLockNotOwned) {
		t.Fatalf("Unlock err = %v, want ErrLockNotOwned", err)
	}
}

func TestRedisLockRenewExtendsTTL(t *testing.T) {
	server, client := newMiniRedisClient(t)
	locker := newTestRedisLocker(t, client)

	lock, ok, err := locker.Acquire(context.Background(), "nightly", 2*time.Second, 0)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !ok {
		t.Fatal("Acquire ok = false, want true")
	}

	server.FastForward(1500 * time.Millisecond)
	if err := lock.Renew(context.Background(), 5*time.Second); err != nil {
		t.Fatalf("Renew: %v", err)
	}

	ttl := server.TTL("aegiscore:cron:nightly")
	if ttl <= 3*time.Second {
		t.Fatalf("ttl = %s, want renewed ttl over 3s", ttl)
	}
}

func TestRedisLockerWaitTimeoutReturnsBusy(t *testing.T) {
	_, client := newMiniRedisClient(t)
	locker := newTestRedisLocker(t, client)

	firstLock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	if !ok {
		t.Fatal("first Acquire ok = false, want true")
	}
	defer func() {
		_ = firstLock.Unlock(context.Background())
	}()

	startedAt := time.Now()
	lock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	if ok || lock != nil {
		t.Fatalf("second Acquire = (%v, %v), want busy", lock, ok)
	}
	if elapsed := time.Since(startedAt); elapsed < 20*time.Millisecond {
		t.Fatalf("wait elapsed = %s, want at least 20ms", elapsed)
	}
}

func TestRedisLockerMaxAttemptsLimitsRetries(t *testing.T) {
	_, client := newMiniRedisClient(t)
	locker, err := NewRedisLocker(client, RedisLockerOptions{
		Namespace: "aegiscore",
		Scope:     []string{"cron"},
		Retry: RetryPolicy{
			InitialInterval: time.Millisecond,
			MaxInterval:     time.Millisecond,
			MaxAttempts:     1,
		},
	})
	if err != nil {
		t.Fatalf("NewRedisLocker: %v", err)
	}

	firstLock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	if !ok {
		t.Fatal("first Acquire ok = false, want true")
	}
	defer func() {
		_ = firstLock.Unlock(context.Background())
	}()

	startedAt := time.Now()
	lock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, time.Second)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	if ok || lock != nil {
		t.Fatalf("second Acquire = (%v, %v), want busy", lock, ok)
	}
	if elapsed := time.Since(startedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("elapsed = %s, want fast return after max attempts", elapsed)
	}
}

func TestRetryPolicyDefaultsAndBackoff(t *testing.T) {
	policy, err := normalizeRetryPolicy(RetryPolicy{})
	if err != nil {
		t.Fatalf("normalizeRetryPolicy: %v", err)
	}
	if policy.InitialInterval != 50*time.Millisecond {
		t.Fatalf("initial interval = %s, want 50ms", policy.InitialInterval)
	}
	if policy.MaxInterval != time.Second {
		t.Fatalf("max interval = %s, want 1s", policy.MaxInterval)
	}

	delay := policy.InitialInterval
	delay = nextRetryDelay(delay, policy.MaxInterval)
	if delay != 100*time.Millisecond {
		t.Fatalf("first backoff = %s, want 100ms", delay)
	}
	delay = nextRetryDelay(800*time.Millisecond, policy.MaxInterval)
	if delay != time.Second {
		t.Fatalf("capped backoff = %s, want 1s", delay)
	}
	delay = nextRetryDelay(time.Second, policy.MaxInterval)
	if delay != time.Second {
		t.Fatalf("max backoff = %s, want 1s", delay)
	}
}

func TestRetryPolicyRejectsInvalidIntervals(t *testing.T) {
	_, err := normalizeRetryPolicy(RetryPolicy{
		InitialInterval: time.Second,
		MaxInterval:     time.Millisecond,
	})
	if !errors.Is(err, ErrInvalidLock) {
		t.Fatalf("normalizeRetryPolicy err = %v, want ErrInvalidLock", err)
	}
}

func newMiniRedisClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("Close redis client: %v", err)
		}
	})
	return server, client
}

func newTestRedisLocker(t *testing.T, client redis.UniversalClient) *RedisLocker {
	t.Helper()
	locker, err := NewRedisLocker(client, RedisLockerOptions{
		Namespace: "aegiscore",
		Scope:     []string{"cron"},
		Retry: RetryPolicy{
			InitialInterval: 5 * time.Millisecond,
			MaxInterval:     5 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewRedisLocker: %v", err)
	}
	return locker
}
