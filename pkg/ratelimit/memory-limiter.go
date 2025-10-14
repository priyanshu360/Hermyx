package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	tokens         int64
	maxTokens      int64
	refillRate     int64         // tokens per second
	lastRefillTime time.Time
	mu             sync.Mutex
}

// MemoryRateLimiter implements in-memory token bucket rate limiting
type MemoryRateLimiter struct {
	buckets   map[string]*TokenBucket
	mu        sync.RWMutex
	maxTokens int64
	window    time.Duration
	ttl       time.Duration // Cleanup TTL for idle buckets
}

// NewMemoryRateLimiter creates a new in-memory rate limiter
func NewMemoryRateLimiter(maxRequests int64, window time.Duration) *MemoryRateLimiter {
	limiter := &MemoryRateLimiter{
		buckets:   make(map[string]*TokenBucket),
		maxTokens: maxRequests,
		window:    window,
		ttl:       window * 2, // Keep buckets around for 2x window duration
	}

	// Start cleanup goroutine
	go limiter.cleanup()

	return limiter
}

// Allow checks if a request should be allowed for the given key
// Returns: allowed (bool), remaining (int64), resetTime (time.Time)
func (m *MemoryRateLimiter) Allow(key string) (bool, int64, time.Time) {
	m.mu.Lock()
	bucket, exists := m.buckets[key]
	if !exists {
		bucket = m.createBucket()
		m.buckets[key] = bucket
	}
	m.mu.Unlock()

	return bucket.consume()
}

// createBucket creates a new token bucket
func (m *MemoryRateLimiter) createBucket() *TokenBucket {
	refillRate := int64(float64(m.maxTokens) / m.window.Seconds())
	if refillRate < 1 {
		refillRate = 1
	}

	return &TokenBucket{
		tokens:         m.maxTokens,
		maxTokens:      m.maxTokens,
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

// consume attempts to consume a token from the bucket
func (tb *TokenBucket) consume() (bool, int64, time.Time) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime)

	// Refill tokens based on elapsed time
	tokensToAdd := int64(elapsed.Seconds()) * tb.refillRate
	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.maxTokens {
			tb.tokens = tb.maxTokens
		}
		tb.lastRefillTime = now
	}

	// Try to consume a token
	if tb.tokens > 0 {
		tb.tokens--
		resetTime := tb.calculateResetTime(now)
		return true, tb.tokens, resetTime
	}

	// Not enough tokens
	resetTime := tb.calculateResetTime(now)
	return false, 0, resetTime
}

// calculateResetTime calculates when the bucket will be fully refilled
func (tb *TokenBucket) calculateResetTime(now time.Time) time.Time {
	if tb.tokens >= tb.maxTokens {
		return now
	}

	tokensNeeded := tb.maxTokens - tb.tokens
	secondsNeeded := float64(tokensNeeded) / float64(tb.refillRate)
	return now.Add(time.Duration(secondsNeeded * float64(time.Second)))
}

// cleanup periodically removes idle buckets
func (m *MemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(m.ttl)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for key, bucket := range m.buckets {
			bucket.mu.Lock()
			idle := now.Sub(bucket.lastRefillTime)
			bucket.mu.Unlock()

			if idle > m.ttl {
				delete(m.buckets, key)
			}
		}
		m.mu.Unlock()
	}
}

// Reset removes the rate limit entry for a specific key
func (m *MemoryRateLimiter) Reset(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.buckets, key)
}

// Close cleans up resources
func (m *MemoryRateLimiter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets = make(map[string]*TokenBucket)
	return nil
}

