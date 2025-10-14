package ratelimit

import (
	"fmt"
	"hermyx/pkg/models"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

const (
	STORAGE_MEMORY = "memory"
	STORAGE_REDIS  = "redis"
)

const (
	KEY_TYPE_IP     = "ip"
	KEY_TYPE_HEADER = "header"
)

// IRateLimiter defines the interface for rate limiting implementations
type IRateLimiter interface {
	Allow(key string) (bool, int64, time.Time)
	Reset(key string)
	Close() error
}

// RateLimiter manages rate limiting for requests
type RateLimiter struct {
	limiter IRateLimiter
	config  *models.RateLimitConfig
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed   bool
	Remaining int64
	ResetTime time.Time
	Limit     int64
	Key       string
}

// NewRateLimiter creates a new rate limiter based on configuration
func NewRateLimiter(config *models.RateLimitConfig) (*RateLimiter, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	// Set defaults
	if config.StatusCode == 0 {
		config.StatusCode = fasthttp.StatusTooManyRequests
	}
	if config.Message == "" {
		config.Message = "Rate limit exceeded. Please try again later."
	}
	if config.Requests <= 0 {
		config.Requests = 100
	}
	if config.Window <= 0 {
		config.Window = 1 * time.Minute
	}
	if config.Storage == "" {
		config.Storage = STORAGE_MEMORY
	}

	var limiter IRateLimiter
	var err error

	switch strings.ToLower(config.Storage) {
	case STORAGE_MEMORY:
		limiter = NewMemoryRateLimiter(config.Requests, config.Window)
	case STORAGE_REDIS:
		if config.Redis == nil {
			return nil, fmt.Errorf("redis configuration required for redis rate limiter")
		}
		limiter = NewRedisRateLimiter(config.Redis, config.Requests, config.Window)
	default:
		return nil, fmt.Errorf("unsupported rate limit storage type: %s", config.Storage)
	}

	return &RateLimiter{
		limiter: limiter,
		config:  config,
	}, err
}

// Check checks if a request should be allowed
func (rl *RateLimiter) Check(ctx *fasthttp.RequestCtx) *RateLimitResult {
	if rl == nil || rl.limiter == nil {
		// Rate limiting is disabled
		return &RateLimitResult{
			Allowed:   true,
			Remaining: -1,
			ResetTime: time.Time{},
			Limit:     -1,
		}
	}

	key := rl.buildKey(ctx)
	allowed, remaining, resetTime := rl.limiter.Allow(key)

	return &RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetTime: resetTime,
		Limit:     rl.config.Requests,
		Key:       key,
	}
}

// buildKey builds a rate limit key based on the configuration
func (rl *RateLimiter) buildKey(ctx *fasthttp.RequestCtx) string {
	if len(rl.config.KeyBy) == 0 {
		// Default to IP
		return getClientIP(ctx)
	}

	var parts []string
	for _, keyType := range rl.config.KeyBy {
		if keyType == KEY_TYPE_IP {
			parts = append(parts, getClientIP(ctx))
		} else if strings.HasPrefix(keyType, KEY_TYPE_HEADER+":") {
			headerName := strings.TrimPrefix(keyType, KEY_TYPE_HEADER+":")
			headerValue := string(ctx.Request.Header.Peek(headerName))
			if headerValue != "" {
				parts = append(parts, headerValue)
			}
		} else {
			// Unknown key type, use as-is
			parts = append(parts, keyType)
		}
	}

	if len(parts) == 0 {
		// Fallback to IP if no valid keys
		return getClientIP(ctx)
	}

	return strings.Join(parts, ":")
}

// getClientIP extracts the client IP from the request
func getClientIP(ctx *fasthttp.RequestCtx) string {
	// Check X-Forwarded-For header first
	xff := string(ctx.Request.Header.Peek("X-Forwarded-For"))
	if xff != "" {
		// Take the first IP in the list
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	xri := string(ctx.Request.Header.Peek("X-Real-IP"))
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return ctx.RemoteIP().String()
}

// Reset resets the rate limit for a specific key
func (rl *RateLimiter) Reset(key string) {
	if rl != nil && rl.limiter != nil {
		rl.limiter.Reset(key)
	}
}

// Close cleans up resources
func (rl *RateLimiter) Close() error {
	if rl != nil && rl.limiter != nil {
		return rl.limiter.Close()
	}
	return nil
}

// Resolve merges route-specific rate limit config with global config
func Resolve(globalConfig *models.RateLimitConfig, routeConfig *models.RateLimitConfig) *models.RateLimitConfig {
	if routeConfig == nil {
		return globalConfig
	}

	if !routeConfig.Enabled {
		return routeConfig
	}

	config := &models.RateLimitConfig{
		Enabled:       routeConfig.Enabled,
		Requests:      routeConfig.Requests,
		Window:        routeConfig.Window,
		Storage:       routeConfig.Storage,
		KeyBy:         routeConfig.KeyBy,
		BlockDuration: routeConfig.BlockDuration,
		StatusCode:    routeConfig.StatusCode,
		Message:       routeConfig.Message,
		Redis:         routeConfig.Redis,
		Headers:       routeConfig.Headers,
	}

	// Inherit from global config if not specified
	if config.Requests == 0 && globalConfig != nil {
		config.Requests = globalConfig.Requests
	}
	if config.Window == 0 && globalConfig != nil {
		config.Window = globalConfig.Window
	}
	if config.Storage == "" && globalConfig != nil {
		config.Storage = globalConfig.Storage
	}
	if len(config.KeyBy) == 0 && globalConfig != nil {
		config.KeyBy = globalConfig.KeyBy
	}
	if config.BlockDuration == 0 && globalConfig != nil {
		config.BlockDuration = globalConfig.BlockDuration
	}
	if config.StatusCode == 0 && globalConfig != nil {
		config.StatusCode = globalConfig.StatusCode
	}
	if config.Message == "" && globalConfig != nil {
		config.Message = globalConfig.Message
	}
	if config.Redis == nil && globalConfig != nil {
		config.Redis = globalConfig.Redis
	}
	if config.Headers == nil && globalConfig != nil {
		config.Headers = globalConfig.Headers
	}

	return config
}

// SetRateLimitHeaders sets the rate limit headers on the response
func SetRateLimitHeaders(ctx *fasthttp.RequestCtx, result *RateLimitResult, config *models.RateLimitConfig) {
	if config == nil || config.Headers == nil {
		// Set default headers
		ctx.Response.Header.Set("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
		ctx.Response.Header.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
		ctx.Response.Header.Set("X-RateLimit-Reset", fmt.Sprintf("%d", result.ResetTime.Unix()))
		return
	}

	if config.Headers.IncludeLimit {
		ctx.Response.Header.Set("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
	}
	if config.Headers.IncludeRemaining {
		ctx.Response.Header.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
	}
	if config.Headers.IncludeReset {
		ctx.Response.Header.Set("X-RateLimit-Reset", fmt.Sprintf("%d", result.ResetTime.Unix()))
	}
}
