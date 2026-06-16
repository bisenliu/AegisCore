package localcache

import (
	"testing"
	"time"
)

func TestCacheGetSetAndExpire(t *testing.T) {
	baseTime := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	now := baseTime
	cache := New[string, int64](time.Second)
	cache.setNowForTest(func() time.Time { return now })

	if _, ok := cache.Get("user-1"); ok {
		t.Fatal("Get before Set = hit, want miss")
	}

	cache.Set("user-1", 7)
	version, ok := cache.Get("user-1")
	if !ok || version != 7 {
		t.Fatalf("Get after Set = (%d, %v), want (7, true)", version, ok)
	}

	now = baseTime.Add(time.Second)
	if _, ok := cache.Get("user-1"); ok {
		t.Fatal("Get after TTL = hit, want miss")
	}
}

func TestCacheDelete(t *testing.T) {
	cache := New[string, int64](time.Second)
	cache.Set("user-1", 7)
	cache.Delete("user-1")

	if _, ok := cache.Get("user-1"); ok {
		t.Fatal("Get after Delete = hit, want miss")
	}
}

func TestCacheFallsBackDefaultTTL(t *testing.T) {
	baseTime := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	now := baseTime
	cache := New[string, int64](0)
	cache.setNowForTest(func() time.Time { return now })
	cache.Set("user-1", 7)

	now = baseTime.Add(999 * time.Millisecond)
	if _, ok := cache.Get("user-1"); !ok {
		t.Fatal("Get before default TTL = miss, want hit")
	}
	now = baseTime.Add(time.Second)
	if _, ok := cache.Get("user-1"); ok {
		t.Fatal("Get at default TTL = hit, want miss")
	}
}
