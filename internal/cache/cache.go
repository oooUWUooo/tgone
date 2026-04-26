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
	mu     sync.RWMutex
	ttl    time.Duration
	m      map[string]entry[V]
	stopCh chan struct{}
}

// New creates a Cache with the given TTL and starts a background cleanup goroutine.
// Call Stop to terminate the goroutine when the cache is no longer needed.
func New[V any](ttl time.Duration) *Cache[V] {
	c := &Cache[V]{
		ttl:    ttl,
		m:      make(map[string]entry[V]),
		stopCh: make(chan struct{}),
	}
	go c.runCleanup()
	return c
}

// Stop terminates the background cleanup goroutine. After Stop, the cache is
// still usable for reads and writes, but expired entries are no longer evicted
// automatically. Calling Stop more than once panics.
func (c *Cache[V]) Stop() {
	close(c.stopCh)
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

// runCleanup evicts expired entries once per TTL cycle until Stop is called.
func (c *Cache[V]) runCleanup() {
	t := time.NewTicker(c.ttl)
	defer t.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
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
}
