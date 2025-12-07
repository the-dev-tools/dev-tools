//nolint:revive // exported
package cachettl

import (
	"sync"
	"time"
)

// Cache provides a concurrency-safe in-memory key/value store with TTL support.
// It is designed to be lightweight and rely on Go generics so callers can store
// arbitrary value types without additional allocations.
type Cache[K comparable, V any] struct {
	mu         sync.RWMutex
	entries    map[K]entry[V]
	defaultTTL time.Duration

	cleanupStop chan struct{}
	cleanupDone chan struct{}
	once        sync.Once
}

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// New creates a cache with the provided default TTL and optional cleanup
// interval. A defaultTTL <= 0 disables expiration for entries inserted via Set.
// A cleanup interval <= 0 means no background cleanup goroutine is spawned.
func New[K comparable, V any](defaultTTL time.Duration, cleanupInterval time.Duration) *Cache[K, V] {
	c := &Cache[K, V]{
		entries:    make(map[K]entry[V]),
		defaultTTL: defaultTTL,
	}

	if cleanupInterval > 0 {
		c.cleanupStop = make(chan struct{})
		c.cleanupDone = make(chan struct{})
		go c.cleanupLoop(cleanupInterval)
	}

	return c
}

// Set stores the key/value pair using the cache's default TTL. A default TTL of
// zero disables expiration for the inserted entry.
func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithTTL(key, value, 0)
}

// SetWithTTL stores the key/value pair with the provided TTL. A ttl <= 0 falls
// back to the cache's default TTL. If both are <= 0, the entry does not expire.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	var expiresAt time.Time

	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	c.mu.Lock()
	c.entries[key] = entry[V]{
		value:     value,
		expiresAt: expiresAt,
	}
	c.mu.Unlock()
}

// Get fetches the value for the given key. It returns false if the key does not
// exist or if the stored value has expired.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	var zero V

	c.mu.RLock()
	ent, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return zero, false
	}

	if ent.expiresAt.IsZero() || ent.expiresAt.After(time.Now()) {
		return ent.value, true
	}

	c.mu.Lock()
	// Ensure the entry is still present and expired before deleting.
	if latest, present := c.entries[key]; present {
		if !latest.expiresAt.IsZero() && !latest.expiresAt.After(time.Now()) {
			delete(c.entries, key)
		}
	}
	c.mu.Unlock()

	return zero, false
}

// Delete removes a key from the cache. It returns true when the key was present.
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	_, ok := c.entries[key]
	if ok {
		delete(c.entries, key)
	}
	c.mu.Unlock()
	return ok
}

// Len returns the number of entries currently stored in the cache (including
// expired ones that have not yet been observed or cleaned).
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	n := len(c.entries)
	c.mu.RUnlock()
	return n
}

// PurgeExpired removes all expired entries and returns the number of entries removed.
func (c *Cache[K, V]) PurgeExpired() int {
	now := time.Now()
	removed := 0

	c.mu.Lock()
	for k, ent := range c.entries {
		if !ent.expiresAt.IsZero() && !ent.expiresAt.After(now) {
			delete(c.entries, k)
			removed++
		}
	}
	c.mu.Unlock()

	return removed
}

// Close stops the background cleanup goroutine, if one was started. The cache
// remains usable after Close; callers can still use Set/Get operations.
func (c *Cache[K, V]) Close() {
	if c.cleanupStop == nil {
		return
	}

	c.once.Do(func() {
		close(c.cleanupStop)
		<-c.cleanupDone
	})
}

func (c *Cache[K, V]) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer func() {
		ticker.Stop()
		close(c.cleanupDone)
	}()

	for {
		select {
		case <-ticker.C:
			c.PurgeExpired()
		case <-c.cleanupStop:
			return
		}
	}
}
