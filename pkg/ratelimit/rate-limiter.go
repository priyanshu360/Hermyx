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
	AllowWithLimit(key string, limit int64, window time.Duration) (bool, int64, time.Time)
	Reset(key string)
	Health() error
	Close() error
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed   bool
	Remaining int64
	ResetTime time.Time
	Limit     int64
	Key       string
}

// NewRateLimiterBackend creates a new rate limiter backend based on global configuration
func NewRateLimiter(config *models.RateLimitConfig) (IRateLimiter, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	// Set defaults only for negative values (invalid)
	if config.Requests < 0 {
		config.Requests = 100
	}
	if config.Window <= 0 {
		config.Window = 1 * time.Minute
	}
	if config.Storage == "" {
		config.Storage = STORAGE_MEMORY
	}

	var limiter IRateLimiter

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

	return limiter, nil
}

// BuildKey builds a rate limit key based on the configuration
func BuildKey(ctx *fasthttp.RequestCtx, config *models.RateLimitConfig) string {
	if config == nil || len(config.KeyBy) == 0 {
		// Default to IP
		return getClientIP(ctx)
	}

	var parts []string
	for _, keyType := range config.KeyBy {
		if keyType == KEY_TYPE_IP {
			parts = append(parts, getClientIP(ctx))
		} else if strings.HasPrefix(keyType, KEY_TYPE_HEADER+":") {
			headerName := strings.TrimPrefix(keyType, KEY_TYPE_HEADER+":")
			headerValue := string(ctx.Request.Header.Peek(headerName))
			if headerValue != "" {
				parts = append(parts, headerValue)
			} else {
				// If required header is missing, return empty string
				return ""
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

// Resolve merges route-specific rate limit config with global config
// IMPORTANT: Storage and Redis are ALWAYS inherited from global config and cannot be overridden per route
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
		KeyBy:         routeConfig.KeyBy,
		BlockDuration: routeConfig.BlockDuration,
		StatusCode:    routeConfig.StatusCode,
		Message:       routeConfig.Message,
		Headers:       routeConfig.Headers,
		// Storage and Redis are ALWAYS inherited from global config
		Storage: "",
		Redis:   nil,
	}

	// Inherit from global config if not specified
	if config.Requests == 0 && globalConfig != nil {
		config.Requests = globalConfig.Requests
	}
	if config.Window == 0 && globalConfig != nil {
		config.Window = globalConfig.Window
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
	if config.Headers == nil && globalConfig != nil {
		config.Headers = globalConfig.Headers
	}

	// ALWAYS inherit storage and redis from global config (cannot be overridden)
	if globalConfig != nil {
		config.Storage = globalConfig.Storage
		config.Redis = globalConfig.Redis
	}

	return config
}
