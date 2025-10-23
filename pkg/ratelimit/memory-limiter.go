package ratelimit

import (
	"hermyx/pkg/utils/logger"
	"sync"
	"time"
)

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	tokens         int64
	maxTokens      int64
	refillRate     int64 // tokens per second
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
	logger    *logger.Logger
}

// NewMemoryRateLimiter creates a new in-memory rate limiter
func NewMemoryRateLimiter(maxRequests int64, window time.Duration, logger *logger.Logger) *MemoryRateLimiter {
	// Logger is always provided - no need for nil checks
	limiter := &MemoryRateLimiter{
		buckets:   make(map[string]*TokenBucket),
		maxTokens: maxRequests,
		window:    window,
		ttl:       window * 2, // Keep buckets around for 2x window duration
		logger:    logger,
	}

	// Start cleanup goroutine
	go limiter.cleanup()

	return limiter
}

// Allow checks if a request should be allowed for the given key
// Returns: allowed (bool), remaining (int64), resetTime (time.Time)
func (m *MemoryRateLimiter) Allow(key string) (bool, int64, time.Time) {
	return m.AllowWithLimit(key, m.maxTokens, m.window)
}

// AllowWithLimit checks if a request should be allowed for the given key with a specific limit and window
// Returns: allowed (bool), remaining (int64), resetTime (time.Time)
func (m *MemoryRateLimiter) AllowWithLimit(key string, limit int64, window time.Duration) (bool, int64, time.Time) {
	m.mu.Lock()
	bucket, exists := m.buckets[key]
	if !exists {
		bucket = m.createBucketWithLimit(limit, window)
		m.buckets[key] = bucket
		m.logger.Debug("Created new token bucket for key")
	}
	m.mu.Unlock()

	refillRate := int64(float64(limit) / window.Seconds())
	if refillRate < 1 {
		refillRate = 1
	}
	allowed, remaining, resetTime := bucket.consumeWithLimit(limit, refillRate)

	if allowed {
		m.logger.Debug("Rate limit check passed")
	} else {
		m.logger.Debug("Rate limit check failed")
	}

	return allowed, remaining, resetTime
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

// createBucketWithLimit creates a new token bucket with specific limit and window
func (m *MemoryRateLimiter) createBucketWithLimit(limit int64, window time.Duration) *TokenBucket {
	refillRate := int64(float64(limit) / window.Seconds())
	if refillRate < 1 {
		refillRate = 1
	}

	return &TokenBucket{
		tokens:         limit,
		maxTokens:      limit,
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

// consume attempts to consume a token from the bucket
func (tb *TokenBucket) consume() (bool, int64, time.Time) {
	return tb.consumeWithLimit(tb.maxTokens, tb.refillRate)
}

// consumeWithLimit attempts to consume a token from the bucket with specific limit and refill rate
func (tb *TokenBucket) consumeWithLimit(limit int64, refillRate int64) (bool, int64, time.Time) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// If limit is 0, immediately block the request
	if limit <= 0 {
		now := time.Now()
		resetTime := tb.calculateResetTimeWithLimit(now, limit, refillRate)
		return false, 0, resetTime
	}

	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime)

	// Refill tokens based on elapsed time
	tokensToAdd := int64(elapsed.Seconds()) * refillRate
	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > limit {
			tb.tokens = limit
		}
		tb.lastRefillTime = now
	}

	// Try to consume a token
	if tb.tokens > 0 {
		tb.tokens--
		resetTime := tb.calculateResetTimeWithLimit(now, limit, refillRate)
		return true, tb.tokens, resetTime
	}

	// Not enough tokens
	resetTime := tb.calculateResetTimeWithLimit(now, limit, refillRate)
	return false, 0, resetTime
}

// calculateResetTimeWithLimit calculates when the bucket will be fully refilled with specific limit and refill rate
func (tb *TokenBucket) calculateResetTimeWithLimit(now time.Time, limit int64, refillRate int64) time.Time {
	if tb.tokens >= limit {
		return now
	}

	tokensNeeded := limit - tb.tokens
	secondsNeeded := float64(tokensNeeded) / float64(refillRate)
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
	m.logger.Debug("Resetting rate limit for key")
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.buckets, key)
}

// Health checks if the memory rate limiter is healthy
func (m *MemoryRateLimiter) Health() error {
	m.logger.Debug("Memory rate limiter health check")
	// Memory rate limiter is always healthy as it doesn't depend on external services
	return nil
}

// Close cleans up resources
func (m *MemoryRateLimiter) Close() error {
	m.logger.Debug("Closing memory rate limiter")
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets = make(map[string]*TokenBucket)
	return nil
}
