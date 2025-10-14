package ratelimit

import (
	"testing"
	"time"
)

func TestMemoryRateLimiter_Allow(t *testing.T) {
	limiter := NewMemoryRateLimiter(10, 1*time.Minute)
	defer limiter.Close()

	key := "test-key"

	// Test that we can make requests up to the limit
	for i := 0; i < 10; i++ {
		allowed, remaining, _ := limiter.Allow(key)
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
		expectedRemaining := int64(9 - i)
		if remaining != expectedRemaining {
			t.Errorf("Expected remaining %d, got %d", expectedRemaining, remaining)
		}
	}

	// 11th request should be blocked
	allowed, remaining, resetTime := limiter.Allow(key)
	if allowed {
		t.Error("11th request should be blocked")
	}
	if remaining != 0 {
		t.Errorf("Expected remaining 0, got %d", remaining)
	}
	if resetTime.Before(time.Now()) {
		t.Error("Reset time should be in the future")
	}
}

func TestMemoryRateLimiter_TokenRefill(t *testing.T) {
	// Use a small window for faster testing
	limiter := NewMemoryRateLimiter(5, 2*time.Second)
	defer limiter.Close()

	key := "test-refill"

	// Consume all tokens
	for i := 0; i < 5; i++ {
		allowed, _, _ := limiter.Allow(key)
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Should be blocked now
	allowed, _, _ := limiter.Allow(key)
	if allowed {
		t.Error("Request should be blocked after consuming all tokens")
	}

	// Wait for tokens to refill
	time.Sleep(2500 * time.Millisecond)

	// Should be allowed again after refill
	allowed, _, _ = limiter.Allow(key)
	if !allowed {
		t.Error("Request should be allowed after token refill")
	}
}

func TestMemoryRateLimiter_MultipleKeys(t *testing.T) {
	limiter := NewMemoryRateLimiter(3, 1*time.Minute)
	defer limiter.Close()

	key1 := "user1"
	key2 := "user2"

	// Consume tokens for key1
	for i := 0; i < 3; i++ {
		allowed, _, _ := limiter.Allow(key1)
		if !allowed {
			t.Errorf("Request %d for key1 should be allowed", i+1)
		}
	}

	// key1 should be blocked
	allowed, _, _ := limiter.Allow(key1)
	if allowed {
		t.Error("key1 should be blocked")
	}

	// key2 should still be allowed (different bucket)
	allowed, _, _ = limiter.Allow(key2)
	if !allowed {
		t.Error("key2 should be allowed")
	}
}

func TestMemoryRateLimiter_Reset(t *testing.T) {
	limiter := NewMemoryRateLimiter(3, 1*time.Minute)
	defer limiter.Close()

	key := "test-reset"

	// Consume all tokens
	for i := 0; i < 3; i++ {
		limiter.Allow(key)
	}

	// Should be blocked
	allowed, _, _ := limiter.Allow(key)
	if allowed {
		t.Error("Request should be blocked")
	}

	// Reset the key
	limiter.Reset(key)

	// Should be allowed again
	allowed, _, _ = limiter.Allow(key)
	if !allowed {
		t.Error("Request should be allowed after reset")
	}
}

func TestMemoryRateLimiter_Cleanup(t *testing.T) {
	limiter := NewMemoryRateLimiter(10, 100*time.Millisecond)
	defer limiter.Close()

	// Create a bucket
	limiter.Allow("test-cleanup")

	// Wait for TTL to expire (2x window = 200ms)
	time.Sleep(300 * time.Millisecond)

	// Check bucket was cleaned up (we can't directly check the internal map,
	// but we can verify behavior is correct)
	allowed, remaining, _ := limiter.Allow("test-cleanup")
	if !allowed {
		t.Error("Request should be allowed after cleanup")
	}
	// After cleanup, a new bucket is created with full tokens
	if remaining != 9 {
		t.Errorf("Expected 9 tokens remaining (new bucket), got %d", remaining)
	}
}

func BenchmarkMemoryRateLimiter_Allow(b *testing.B) {
	limiter := NewMemoryRateLimiter(10000, 1*time.Minute)
	defer limiter.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow("benchmark-key")
	}
}

func BenchmarkMemoryRateLimiter_AllowMultipleKeys(b *testing.B) {
	limiter := NewMemoryRateLimiter(10000, 1*time.Minute)
	defer limiter.Close()

	keys := []string{"key1", "key2", "key3", "key4", "key5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		limiter.Allow(key)
	}
}

