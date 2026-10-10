package aicheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DefaultCachePath is the judge cache, relative to the working directory. The key is the
// Stresseur CLI's, so both can share one file (Stresseur keeps .stresseur/judge-cache.json).
const DefaultCachePath = ".devtools/judge-cache.json"

// MaxCacheEntries bounds the cache file; the oldest entries go first.
const MaxCacheEntries = 5000

// CacheKey identifies a verdict: same judge, same rubric, same data → same verdict.
func CacheKey(cfg JudgeConfig, j *JudgeSpec, input, output string) string {
	b, _ := json.Marshal([]any{"v1", cfg.Provider, cfg.Model, cfg.BaseURL, cfg.Samples,
		j.Criteria, j.Steps, j.MinScore, input, output})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// CacheEntry is one stored verdict.
type CacheEntry struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
	Model  string  `json:"model"`
	At     string  `json:"at"`
}

// Cache is the verdict cache, kept in a JSON file. Safe for concurrent use.
type Cache struct {
	path    string
	mu      sync.Mutex
	entries map[string]CacheEntry
	dirty   bool
}

// OpenCache reads the cache file; a missing or unreadable file is an empty cache. path "" keeps
// it in memory only.
func OpenCache(path string) *Cache {
	c := &Cache{path: path, entries: map[string]CacheEntry{}}
	if path == "" {
		return c
	}
	if b, err := os.ReadFile(filepath.Clean(path)); err == nil {
		var doc struct {
			Version int                   `json:"version"`
			Entries map[string]CacheEntry `json:"entries"`
		}
		if json.Unmarshal(b, &doc) == nil && doc.Version == 1 && doc.Entries != nil {
			c.entries = doc.Entries
		}
	}
	return c
}

// Get returns a stored verdict.
func (c *Cache) Get(key string) (CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	return e, ok
}

// Put stores a verdict.
func (c *Cache) Put(key string, e CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e.At == "" {
		e.At = time.Now().UTC().Format(time.RFC3339)
	}
	c.entries[key] = e
	c.dirty = true
}

// Len is the number of stored verdicts.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Save writes the file (atomically) when something changed.
func (c *Cache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == "" || !c.dirty {
		return nil
	}
	if len(c.entries) > MaxCacheEntries {
		keys := make([]string, 0, len(c.entries))
		for k := range c.entries {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(a, b int) bool { return c.entries[keys[a]].At < c.entries[keys[b]].At })
		for _, k := range keys[:len(keys)-MaxCacheEntries] {
			delete(c.entries, k)
		}
	}
	b, err := json.Marshal(map[string]any{"version": 1, "entries": c.entries})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o750); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.path); err != nil {
		return err
	}
	c.dirty = false
	return nil
}
