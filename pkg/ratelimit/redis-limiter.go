package ratelimit

import (
	"context"
	"fmt"
	"hermyx/pkg/models"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter implements distributed rate limiting using Redis
// It includes configurable fail-open/fail-closed behavior
// When Redis is unavailable:
// - failOpen=true (default): Allow requests to proceed (better availability)
// - failOpen=false: Block all requests (better security)
type RedisRateLimiter struct {
	client    *redis.Client
	namespace string
	maxTokens int64
	window    time.Duration
	ctx       context.Context
	failOpen  bool // If true, allow requests when Redis is down; if false, block them
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(config *models.RedisConfig, maxRequests int64, window time.Duration) *RedisRateLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB: func() int {
			if config.DB == nil {
				return 0
			}
			return *config.DB
		}(),
	})

	namespace := config.KeyNamespace
	if namespace == "" {
		namespace = "hermyx:ratelimit:"
	} else if namespace[len(namespace)-1] != ':' {
		namespace += ":"
	}

	// Default to fail open for better availability
	failOpen := true
	if config.FailOpen != nil {
		failOpen = *config.FailOpen
	}

	return &RedisRateLimiter{
		client:    client,
		namespace: namespace,
		maxTokens: maxRequests,
		window:    window,
		ctx:       context.Background(),
		failOpen:  failOpen,
	}
}

// Allow checks if a request should be allowed for the given key using token bucket algorithm
// Returns: allowed (bool), remaining (int64), resetTime (time.Time)
func (r *RedisRateLimiter) Allow(key string) (bool, int64, time.Time) {
	return r.AllowWithLimit(key, r.maxTokens, r.window)
}

// AllowWithLimit checks if a request should be allowed for the given key with specific limit and window
// Returns: allowed (bool), remaining (int64), resetTime (time.Time)
func (r *RedisRateLimiter) AllowWithLimit(key string, limit int64, window time.Duration) (bool, int64, time.Time) {
	now := time.Now()

	// If limit is 0, immediately block the request
	if limit <= 0 {
		return false, 0, now.Add(window)
	}

	fullKey := r.key(key)
	nowUnix := now.Unix()

	// Lua script for atomic token bucket implementation
	// This ensures consistency in a distributed environment
	script := `
		local key = KEYS[1]
		local max_tokens = tonumber(ARGV[1])
		local refill_rate = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local window = tonumber(ARGV[4])
		
		-- Get current state
		local state = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(state[1]) or max_tokens
		local last_refill = tonumber(state[2]) or now
		
		-- Calculate tokens to add based on time elapsed
		local elapsed = now - last_refill
		local tokens_to_add = math.floor(elapsed * refill_rate)
		
		if tokens_to_add > 0 then
			tokens = math.min(tokens + tokens_to_add, max_tokens)
			last_refill = now
		end
		
		-- Try to consume a token
		local allowed = 0
		if tokens > 0 then
			tokens = tokens - 1
			allowed = 1
		end
		
		-- Update state
		redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
		redis.call('EXPIRE', key, window * 2)
		
		-- Calculate reset time
		local tokens_needed = max_tokens - tokens
		local seconds_to_reset = 0
		if tokens_needed > 0 and refill_rate > 0 then
			seconds_to_reset = math.ceil(tokens_needed / refill_rate)
		end
		
		return {allowed, tokens, now + seconds_to_reset}
	`

	refillRate := float64(limit) / window.Seconds()
	if refillRate < 0.01 {
		refillRate = 0.01
	}

	// Use a timeout context for the Redis operation
	ctx, cancel := context.WithTimeout(r.ctx, 500*time.Millisecond)
	defer cancel()

	result, err := r.client.Eval(ctx, script, []string{fullKey},
		limit,
		refillRate,
		nowUnix,
		int64(window.Seconds()),
	).Result()

	if err != nil {
		// Redis operation failed, decide based on failOpen setting
		if r.failOpen {
			// Fail open: allow the request
			return true, limit, now.Add(window)
		}
		// Fail closed: block the request
		return false, 0, now.Add(window)
	}

	values, ok := result.([]interface{})
	if !ok || len(values) < 3 {
		// Invalid response, decide based on failOpen setting
		if r.failOpen {
			return true, limit, now.Add(window)
		}
		return false, 0, now.Add(window)
	}

	allowed := values[0].(int64) == 1
	remaining := values[1].(int64)
	resetTimestamp := values[2].(int64)
	resetTime := time.Unix(resetTimestamp, 0)

	return allowed, remaining, resetTime
}

// AllowN checks if N requests should be allowed for the given key
// This is useful for bulk operations
func (r *RedisRateLimiter) AllowN(key string, n int64) (bool, int64, time.Time) {
	now := time.Now()

	if n <= 0 {
		return true, r.maxTokens, now
	}

	fullKey := r.key(key)
	nowUnix := now.Unix()

	script := `
		local key = KEYS[1]
		local max_tokens = tonumber(ARGV[1])
		local refill_rate = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local window = tonumber(ARGV[4])
		local n = tonumber(ARGV[5])
		
		-- Get current state
		local state = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(state[1]) or max_tokens
		local last_refill = tonumber(state[2]) or now
		
		-- Calculate tokens to add
		local elapsed = now - last_refill
		local tokens_to_add = math.floor(elapsed * refill_rate)
		
		if tokens_to_add > 0 then
			tokens = math.min(tokens + tokens_to_add, max_tokens)
			last_refill = now
		end
		
		-- Try to consume n tokens
		local allowed = 0
		if tokens >= n then
			tokens = tokens - n
			allowed = 1
		end
		
		-- Update state
		redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
		redis.call('EXPIRE', key, window * 2)
		
		-- Calculate reset time
		local tokens_needed = max_tokens - tokens
		local seconds_to_reset = 0
		if tokens_needed > 0 and refill_rate > 0 then
			seconds_to_reset = math.ceil(tokens_needed / refill_rate)
		end
		
		return {allowed, tokens, now + seconds_to_reset}
	`

	refillRate := float64(r.maxTokens) / r.window.Seconds()
	if refillRate < 0.01 {
		refillRate = 0.01
	}

	// Use a timeout context for the Redis operation
	ctx, cancel := context.WithTimeout(r.ctx, 500*time.Millisecond)
	defer cancel()

	result, err := r.client.Eval(ctx, script, []string{fullKey},
		r.maxTokens,
		refillRate,
		nowUnix,
		int64(r.window.Seconds()),
		n,
	).Result()

	if err != nil {
		// Redis operation failed, decide based on failOpen setting
		if r.failOpen {
			return true, r.maxTokens, now.Add(r.window)
		}
		return false, 0, now.Add(r.window)
	}

	values, ok := result.([]interface{})
	if !ok || len(values) < 3 {
		if r.failOpen {
			return true, r.maxTokens, now.Add(r.window)
		}
		return false, 0, now.Add(r.window)
	}

	allowed := values[0].(int64) == 1
	remaining := values[1].(int64)
	resetTimestamp := values[2].(int64)
	resetTime := time.Unix(resetTimestamp, 0)

	return allowed, remaining, resetTime
}

// key generates the full Redis key with namespace
func (r *RedisRateLimiter) key(k string) string {
	return fmt.Sprintf("%s%s", r.namespace, k)
}

// Health checks if the Redis rate limiter is healthy
func (r *RedisRateLimiter) Health() error {
	return r.Ping()
}

// Ping checks if the Redis connection is alive
func (r *RedisRateLimiter) Ping() error {
	ctx, cancel := context.WithTimeout(r.ctx, 2*time.Second)
	defer cancel()
	return r.client.Ping(ctx).Err()
}

// Reset removes the rate limit entry for a specific key
func (r *RedisRateLimiter) Reset(key string) {
	ctx, cancel := context.WithTimeout(r.ctx, 1*time.Second)
	defer cancel()
	r.client.Del(ctx, r.key(key))
}

// GetLimit returns the current limit and remaining tokens for a key
func (r *RedisRateLimiter) GetLimit(key string) (int64, int64, error) {
	fullKey := r.key(key)

	ctx, cancel := context.WithTimeout(r.ctx, 1*time.Second)
	defer cancel()

	result, err := r.client.HGet(ctx, fullKey, "tokens").Result()
	if err == redis.Nil {
		return r.maxTokens, r.maxTokens, nil
	} else if err != nil {
		return 0, 0, err
	}

	remaining, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		return r.maxTokens, r.maxTokens, nil
	}

	return r.maxTokens, remaining, nil
}

// Close closes the Redis connection and stops the health check goroutine
func (r *RedisRateLimiter) Close() error {
	// Note: The health check goroutine will stop when the client is closed
	return r.client.Close()
}
