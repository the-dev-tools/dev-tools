package cachettl

import (
	"runtime"
	"testing"
	"time"
)

func TestCacheSetGet(t *testing.T) {
	t.Parallel()

	cache := New[string, int](time.Minute, 0)
	cache.Set("alpha", 42)

	if got, ok := cache.Get("alpha"); !ok || got != 42 {
		t.Fatalf("expected value=42, ok=true; got value=%d, ok=%v", got, ok)
	}

	if _, ok := cache.Get("missing"); ok {
		t.Fatal("expected missing key to return ok=false")
	}
}

func TestCacheSetWithTTL(t *testing.T) {
	t.Parallel()

	cache := New[string, string](0, 0)
	cache.SetWithTTL("k", "v", 20*time.Millisecond)

	if val, ok := cache.Get("k"); !ok || val != "v" {
		t.Fatalf("expected cached value before expiration, got %q, ok=%v", val, ok)
	}

	time.Sleep(30 * time.Millisecond)
	if _, ok := cache.Get("k"); ok {
		t.Fatal("expected key to expire")
	}
}

func TestCacheDefaultTTL(t *testing.T) {
	t.Parallel()

	cache := New[string, int](25*time.Millisecond, 0)
	cache.Set("default", 7)

	if _, ok := cache.Get("default"); !ok {
		t.Fatal("expected key immediately after set")
	}

	time.Sleep(40 * time.Millisecond)
	if _, ok := cache.Get("default"); ok {
		t.Fatal("expected default TTL expiration")
	}
}

func TestCacheDelete(t *testing.T) {
	t.Parallel()

	cache := New[string, int](time.Minute, 0)
	cache.Set("z", 1)

	if !cache.Delete("z") {
		t.Fatal("expected delete to return true")
	}
	if cache.Delete("z") {
		t.Fatal("expected delete on missing key to return false")
	}
	if _, ok := cache.Get("z"); ok {
		t.Fatal("expected key to be removed")
	}
}

func TestCachePurgeExpired(t *testing.T) {
	t.Parallel()

	cache := New[string, int](time.Millisecond*10, 0)
	cache.Set("a", 1)
	cache.SetWithTTL("b", 2, time.Millisecond*5)

	time.Sleep(15 * time.Millisecond)
	removed := cache.PurgeExpired()
	if removed == 0 {
		t.Fatal("expected purge to remove entries")
	}
}

func TestCacheCleanupLoop(t *testing.T) {
	cache := New[string, int](time.Millisecond*5, time.Millisecond*5)
	cache.Set("loop", 9)

	time.Sleep(20 * time.Millisecond)

	// Allow finalizers to run to make sure cleanup loop had a chance.
	runtime.Gosched()

	if _, ok := cache.Get("loop"); ok {
		t.Fatal("expected background cleanup to evict entry")
	}

	cache.Close()
	cache.Close() // second close should be a no-op
}
