package localcache

import (
	"testing"
	"time"
)

func TestCacheGetSetAndExpire(t *testing.T) {
	cache := New[string, int64](time.Second)

	if _, ok := cache.Get("user-1"); ok {
		t.Fatal("Get before Set = hit, want miss")
	}

	cache.Set("user-1", 7)
	version, ok := cache.Get("user-1")
	if !ok || version != 7 {
		t.Fatalf("Get after Set = (%d, %v), want (7, true)", version, ok)
	}

	cache.values.Store("user-1", entry[int64]{value: 7, expiresAt: time.Now().Add(-time.Nanosecond)})
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
	cache := New[string, int64](0)
	if cache.ttl != time.Second {
		t.Fatalf("ttl = %s, want 1s", cache.ttl)
	}
	cache.Set("user-1", 7)

	if version, ok := cache.Get("user-1"); !ok || version != 7 {
		t.Fatal("Get before default TTL = miss, want hit")
	}
}
