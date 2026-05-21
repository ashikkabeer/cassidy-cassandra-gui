package schema

import (
	"strings"
	"sync"
	"time"
)

// ttlCache is a tiny in-memory cache with per-entry expiry. Concurrency-safe.
// Schema responses are small (KB-range) and re-fetching is cheap, so a richer
// LRU or weighted cache would be overkill. The cache shrinks on read (lazy
// expiry) so a long-running process won't unbounded-grow even if entries are
// never invalidated explicitly.
type ttlCache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]ttlEntry
}

type ttlEntry struct {
	value     any
	expiresAt time.Time
}

func newTTLCache(ttl time.Duration) *ttlCache {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &ttlCache{ttl: ttl, m: map[string]ttlEntry{}}
}

func (c *ttlCache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.m, key)
		return nil, false
	}
	return e.value, true
}

func (c *ttlCache) set(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = ttlEntry{value: v, expiresAt: time.Now().Add(c.ttl)}
}

// invalidatePrefix drops every entry whose key starts with prefix. Used when
// a connection is updated/deleted to wipe all of its cached introspection.
func (c *ttlCache) invalidatePrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.m {
		if strings.HasPrefix(k, prefix) {
			delete(c.m, k)
		}
	}
}
