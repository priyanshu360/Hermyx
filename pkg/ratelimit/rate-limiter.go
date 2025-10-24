package ratelimit

import (
	"fmt"
	"hermyx/pkg/models"
	"hermyx/pkg/utils/logger"
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
// SetDefaults populates a RateLimitConfig with sensible defaults for any missing or nil fields.
// If config is nil the function returns immediately.
// Defaults applied:
//   - Requests: 100
//   - Window: 1 minute
//   - BlockDuration: 1 minute
//   - StatusCode: 429
//   - Storage: "memory"
//   - KeyBy: []string{"ip"}
//   - Message: "Rate limit exceeded"
func SetDefaults(config *models.RateLimitConfig) {
	if config == nil {
		return
	}

	// Set defaults for nil pointer fields
	if config.Requests == nil {
		requests := int64(100)
		config.Requests = &requests
	}
	if config.Window == nil {
		window := time.Minute
		config.Window = &window
	}
	if config.BlockDuration == nil {
		blockDuration := time.Minute
		config.BlockDuration = &blockDuration
	}
	if config.StatusCode == nil {
		statusCode := 429
		config.StatusCode = &statusCode
	}

	// Set defaults for non-pointer fields
	if config.Storage == "" {
		config.Storage = STORAGE_MEMORY
	}
	if len(config.KeyBy) == 0 {
		config.KeyBy = []string{"ip"}
	}
	if config.Message == "" {
		config.Message = "Rate limit exceeded"
	}
}

// NewRateLimiter creates an IRateLimiter instance using the provided rate limit configuration and logger.
// If config is nil or config.Enabled is false the function returns (nil, nil).
// The configuration is defaulted via SetDefaults before use. Supported storage types:
// - STORAGE_MEMORY: returns an in-memory limiter.
// - STORAGE_REDIS: returns a Redis-backed limiter and requires config.Redis to be non-nil.
// An error is returned if Redis is selected but config.Redis is missing, or if the storage type is unsupported.
func NewRateLimiter(config *models.RateLimitConfig, logger *logger.Logger) (IRateLimiter, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	// Set defaults before using the config
	SetDefaults(config)

	var limiter IRateLimiter

	switch strings.ToLower(config.Storage) {
	case STORAGE_MEMORY:
		limiter = NewMemoryRateLimiter(*config.Requests, *config.Window, logger)
	case STORAGE_REDIS:
		if config.Redis == nil {
			return nil, fmt.Errorf("redis configuration required for redis rate limiter")
		}
		limiter = NewRedisRateLimiter(config.Redis, *config.Requests, *config.Window, logger)
	default:
		return nil, fmt.Errorf("unsupported rate limit storage type: %s", config.Storage)
	}

	return limiter, nil
}

// BuildKey constructs a rate-limit key for the given request according to the provided configuration.
// It composes key parts from config.KeyBy (supported values: "ip" and "header:<name>"), joining parts with ":".
// If config is nil or config.KeyBy is empty, the client's IP is returned.
// For header entries, the header value is used when present; if a required header is missing the client's IP is used instead.
// If no parts are produced, the function falls back to the client's IP.
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
				// Required header missing; fall back to IP to avoid empty key
				parts = append(parts, getClientIP(ctx))
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

// getClientIP extracts the client's IP address from the request context.
// It prefers the first entry of the `X-Forwarded-For` header, falls back to `X-Real-IP`, and uses the request's remote address if neither header is present.
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
// Resolve merges a route-level rate limit configuration with a global configuration.
// If routeConfig is nil, Resolve returns globalConfig. If routeConfig exists but Enabled is false, it returns routeConfig unchanged.
// For an enabled routeConfig, values present in routeConfig are preserved while missing values are inherited from globalConfig for: Requests, Window, KeyBy, BlockDuration, StatusCode, Message, and Headers.
// Storage and Redis are always taken from globalConfig and cannot be overridden per route.
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
	if config.Requests == nil && globalConfig != nil && globalConfig.Requests != nil {
		config.Requests = globalConfig.Requests
	}
	if config.Window == nil && globalConfig != nil && globalConfig.Window != nil {
		config.Window = globalConfig.Window
	}
	if len(config.KeyBy) == 0 && globalConfig != nil {
		config.KeyBy = globalConfig.KeyBy
	}
	if config.BlockDuration == nil && globalConfig != nil && globalConfig.BlockDuration != nil {
		config.BlockDuration = globalConfig.BlockDuration
	}
	if config.StatusCode == nil && globalConfig != nil && globalConfig.StatusCode != nil {
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