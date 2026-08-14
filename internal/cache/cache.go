package cache

import (
	"sync"
	"time"
)

// CachedItem represents an item in the cache with expiration
type CachedItem struct {
	Value     interface{}
	ExpiresAt time.Time
}

// ArticleCache provides thread-safe caching for articles
type ArticleCache struct {
	items map[string]CachedItem
	mux   sync.RWMutex
}

// NewArticleCache creates a new article cache
func NewArticleCache() *ArticleCache {
	return &ArticleCache{
		items: make(map[string]CachedItem),
	}
}

// Set adds an item to the cache with expiration
func (c *ArticleCache) Set(key string, value interface{}, expiry time.Duration) {
	c.mux.Lock()
	defer c.mux.Unlock()
	
	c.items[key] = CachedItem{
		Value:     value,
		ExpiresAt: time.Now().Add(expiry),
	}
}

// Get retrieves an item from the cache
func (c *ArticleCache) Get(key string) (interface{}, bool) {
	c.mux.RLock()
	defer c.mux.RUnlock()
	
	item, exists := c.items[key]
	if !exists {
		return nil, false
	}
	
	if time.Now().After(item.ExpiresAt) {
		return nil, false
	}
	
	return item.Value, true
}

// Delete removes an item from the cache
func (c *ArticleCache) Delete(key string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	
	delete(c.items, key)
}

// Cleanup removes all expired items
func (c *ArticleCache) Cleanup() int {
	c.mux.Lock()
	defer c.mux.Unlock()
	
	count := 0
	now := time.Now()
	for key, item := range c.items {
		if now.After(item.ExpiresAt) {
			delete(c.items, key)
			count++
		}
	}
	
	return count
}

// Size returns the number of items in the cache
func (c *ArticleCache) Size() int {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return len(c.items)
}

// Clear removes all items from the cache
func (c *ArticleCache) Clear() {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.items = make(map[string]CachedItem)
}
