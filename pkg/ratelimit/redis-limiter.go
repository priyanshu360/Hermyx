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
type RedisRateLimiter struct {
	client     *redis.Client
	namespace  string
	maxTokens  int64
	window     time.Duration
	ctx        context.Context
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(config *models.RedisConfig, maxRequests int64, window time.Duration) *RedisRateLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       *config.DB,
	})

	namespace := config.KeyNamespace
	if namespace == "" {
		namespace = "hermyx:ratelimit:"
	} else if namespace[len(namespace)-1] != ':' {
		namespace += ":"
	}

	return &RedisRateLimiter{
		client:     client,
		namespace:  namespace,
		maxTokens:  maxRequests,
		window:     window,
		ctx:        context.Background(),
	}
}

// Allow checks if a request should be allowed for the given key using token bucket algorithm
// Returns: allowed (bool), remaining (int64), resetTime (time.Time)
func (r *RedisRateLimiter) Allow(key string) (bool, int64, time.Time) {
	fullKey := r.key(key)
	now := time.Now()
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

	refillRate := float64(r.maxTokens) / r.window.Seconds()
	if refillRate < 0.01 {
		refillRate = 0.01
	}

	result, err := r.client.Eval(r.ctx, script, []string{fullKey},
		r.maxTokens,
		refillRate,
		nowUnix,
		int64(r.window.Seconds()),
	).Result()

	if err != nil {
		// On error, allow the request (fail open)
		return true, r.maxTokens, now.Add(r.window)
	}

	values, ok := result.([]interface{})
	if !ok || len(values) < 3 {
		// Invalid response, allow the request
		return true, r.maxTokens, now.Add(r.window)
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
	if n <= 0 {
		return true, r.maxTokens, time.Now()
	}

	fullKey := r.key(key)
	now := time.Now()
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

	result, err := r.client.Eval(r.ctx, script, []string{fullKey},
		r.maxTokens,
		refillRate,
		nowUnix,
		int64(r.window.Seconds()),
		n,
	).Result()

	if err != nil {
		return true, r.maxTokens, now.Add(r.window)
	}

	values, ok := result.([]interface{})
	if !ok || len(values) < 3 {
		return true, r.maxTokens, now.Add(r.window)
	}

	allowed := values[0].(int64) == 1
	remaining := values[1].(int64)
	resetTimestamp := values[2].(int64)
	resetTime := time.Unix(resetTimestamp, 0)

	return allowed, remaining, resetTime
}

// Reset removes the rate limit entry for a specific key
func (r *RedisRateLimiter) Reset(key string) {
	r.client.Del(r.ctx, r.key(key))
}

// GetLimit returns the current limit and remaining tokens for a key
func (r *RedisRateLimiter) GetLimit(key string) (int64, int64, error) {
	fullKey := r.key(key)
	
	result, err := r.client.HGet(r.ctx, fullKey, "tokens").Result()
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

// key generates the full Redis key with namespace
func (r *RedisRateLimiter) key(k string) string {
	return fmt.Sprintf("%s%s", r.namespace, k)
}

// Ping checks if the Redis connection is alive
func (r *RedisRateLimiter) Ping() error {
	return r.client.Ping(r.ctx).Err()
}

// Close closes the Redis connection
func (r *RedisRateLimiter) Close() error {
	return r.client.Close()
}

