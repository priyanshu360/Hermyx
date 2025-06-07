package cache

import (
	"sync"
	"testing"
	"time"
)

// Test basic Set and Get operations with TTL.
func TestCache_SetGet(t *testing.T) {
	cache := NewCache(2)
	cache.Set("key1", "value1", 100*time.Millisecond)

	val, ok := cache.Get("key1")
	if !ok || val != "value1" {
		t.Errorf("Expected to get 'value1', got %v, ok=%v", val, ok)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)
	val, ok = cache.Get("key1")
	if ok {
		t.Errorf("Expected key1 to expire, but got value %v", val)
	}
}

// Test eviction when capacity is reached (LRU eviction)
func TestCache_Eviction(t *testing.T) {
	cache := NewCache(2)
	cache.Set("key1", "val1", time.Second)
	cache.Set("key2", "val2", time.Second)

	// Access key1 to make key2 the LRU
	_, _ = cache.Get("key1")

	// Insert key3, should evict key2
	cache.Set("key3", "val3", time.Second)

	if _, ok := cache.Get("key2"); ok {
		t.Error("Expected key2 to be evicted")
	}

	if _, ok := cache.Get("key1"); !ok {
		t.Error("Expected key1 to still exist")
	}

	if _, ok := cache.Get("key3"); !ok {
		t.Error("Expected key3 to exist")
	}
}

// Test Delete method
func TestCache_Delete(t *testing.T) {
	cache := NewCache(2)
	cache.Set("key1", "val1", time.Second)
	cache.Delete("key1")

	if _, ok := cache.Get("key1"); ok {
		t.Error("Expected key1 to be deleted")
	}
}

// Test Len method
func TestCache_Len(t *testing.T) {
	cache := NewCache(2)
	if cache.Len() != 0 {
		t.Error("Expected empty cache length 0")
	}
	cache.Set("key1", "val1", time.Second)
	if cache.Len() != 1 {
		t.Error("Expected cache length 1")
	}
	cache.Set("key2", "val2", time.Second)
	if cache.Len() != 2 {
		t.Error("Expected cache length 2")
	}
	cache.Set("key3", "val3", time.Second) // evict one
	if cache.Len() != 2 {
		t.Error("Expected cache length 2 after eviction")
	}
}

// Simulated concurrency test: multiple goroutines set/get/delete concurrently
func TestCache_ConcurrentAccess(t *testing.T) {
	cache := NewCache(100)

	var wg sync.WaitGroup

	setFn := func(i int) {
		defer wg.Done()
		key := "key" + string(rune(i))
		cache.Set(key, i, time.Second)
	}

	getFn := func(i int) {
		defer wg.Done()
		key := "key" + string(rune(i))
		cache.Get(key)
	}

	deleteFn := func(i int) {
		defer wg.Done()
		key := "key" + string(rune(i))
		cache.Delete(key)
	}

	// Spawn goroutines
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go setFn(i)
		go getFn(i)
		go deleteFn(i)
	}

	wg.Wait()

	// Just check no panic and len <= capacity
	if cache.Len() > 100 {
		t.Errorf("Cache length exceeded capacity: %d", cache.Len())
	}
}
