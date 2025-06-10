package cache

import (
	"testing"
	"time"
)

func FuzzCache_FullBehavior(f *testing.F) {
	f.Add("alpha", "value1", int64(1000))
	f.Add("beta", "value2", int64(200))
	f.Add("gamma", "value3", int64(0))

	f.Fuzz(func(t *testing.T, key, value string, ttlMs int64) {
		cache := NewCache(10)
		ttl := time.Duration(ttlMs%10000) * time.Millisecond

		cache.Set(key, value, ttl)

		// Test retrieval before expiry
		v, ok, _ := cache.Get(key)
		if ttl > 0 && ok && v == nil {
			t.Errorf("Expected non-nil for key=%q before TTL expired", key)
		}

		// Wait until TTL expires (only for small TTLs to avoid slow tests)
		if ttl < 50*time.Millisecond && ttl > 0 {
			time.Sleep(ttl + 10*time.Millisecond)
			if _, ok, _ := cache.Get(key); ok {
				t.Errorf("Expected key=%q to expire after TTL", key)
			}
		}

		// Ensure deletion works
		cache.Delete(key)
		if _, ok, _ := cache.Get(key); ok {
			t.Errorf("Expected key=%q to be deleted", key)
		}
	})
}
