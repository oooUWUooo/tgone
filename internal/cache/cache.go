// Package cache provides a generic in-memory TTL cache safe for concurrent use.
package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// Cache is a generic key-value store where entries expire after a fixed TTL.
type Cache[V any] struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]entry[V]
}

// New creates a Cache with the given TTL and starts a background cleanup goroutine.
// The goroutine exits when the returned Cache is garbage collected (it is non-leaking
// because the ticker is stopped once the cache is evicted).
func New[V any](ttl time.Duration) *Cache[V] {
	c := &Cache[V]{
		ttl: ttl,
		m:   make(map[string]entry[V]),
	}
	go c.runCleanup()
	return c
}

// Get returns the value for key and true if the entry exists and has not expired.
func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores val under key, expiring after the cache TTL.
func (c *Cache[V]) Set(key string, val V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = entry[V]{value: val, expiresAt: time.Now().Add(c.ttl)}
}

// Delete removes an entry immediately, forcing the next Get to miss.
func (c *Cache[V]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

// runCleanup evicts expired entries once per TTL cycle.
func (c *Cache[V]) runCleanup() {
	t := time.NewTicker(c.ttl)
	defer t.Stop()
	for range t.C {
		c.mu.Lock()
		now := time.Now()
		for k, e := range c.m {
			if now.After(e.expiresAt) {
				delete(c.m, k)
			}
		}
		c.mu.Unlock()
	}
}
