package cache

import (
	"testing"
	"time"
)

func TestNewArticleCache(t *testing.T) {
	cache := NewArticleCache()
	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}
	if cache.items == nil {
		t.Fatal("Expected items map to be initialized")
	}
}

func TestSetAndGet(t *testing.T) {
	cache := NewArticleCache()
	
	key := "test-key"
	value := "test-value"
	expiry := 1 * time.Hour
	
	cache.Set(key, value, expiry)
	
	got, found := cache.Get(key)
	if !found {
		t.Fatal("Expected to find key in cache")
	}
	if got != value {
		t.Errorf("Expected %v, got %v", value, got)
	}
}

func TestGetExpired(t *testing.T) {
	cache := NewArticleCache()
	
	key := "test-key"
	value := "test-value"
	expiry := 100 * time.Millisecond
	
	cache.Set(key, value, expiry)
	
	// Wait for expiration
	time.Sleep(150 * time.Millisecond)
	
	_, found := cache.Get(key)
	if found {
		t.Error("Expected key to be expired")
	}
}

func TestDelete(t *testing.T) {
	cache := NewArticleCache()
	
	key := "test-key"
	value := "test-value"
	
	cache.Set(key, value, 1*time.Hour)
	cache.Delete(key)
	
	_, found := cache.Get(key)
	if found {
		t.Error("Expected key to be deleted")
	}
}

func TestCleanup(t *testing.T) {
	cache := NewArticleCache()
	
	// Add one item that will expire
	cache.Set("expire-soon", "value1", 50*time.Millisecond)
	
	// Add one item that won't expire
	cache.Set("expire-later", "value2", 1*time.Hour)
	
	// Wait for first item to expire
	time.Sleep(100 * time.Millisecond)
	
	removed := cache.Cleanup()
	if removed != 1 {
		t.Errorf("Expected 1 item removed, got %d", removed)
	}
	
	// Verify only expired item was removed
	_, found := cache.Get("expire-soon")
	if found {
		t.Error("Expected expired item to be removed")
	}
	
	_, found = cache.Get("expire-later")
	if !found {
		t.Error("Expected non-expired item to remain")
	}
}

func TestSize(t *testing.T) {
	cache := NewArticleCache()
	
	if cache.Size() != 0 {
		t.Errorf("Expected size 0, got %d", cache.Size())
	}
	
	cache.Set("key1", "value1", 1*time.Hour)
	cache.Set("key2", "value2", 1*time.Hour)
	
	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}
}

func TestClear(t *testing.T) {
	cache := NewArticleCache()
	
	cache.Set("key1", "value1", 1*time.Hour)
	cache.Set("key2", "value2", 1*time.Hour)
	
	cache.Clear()
	
	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}
}

func TestConcurrentAccess(t *testing.T) {
	cache := NewArticleCache()
	done := make(chan bool)
	
	// Start multiple goroutines writing to cache
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('a' + id))
				cache.Set(key, j, 1*time.Hour)
				cache.Get(key)
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// If we reach here without panic, test passed
}
