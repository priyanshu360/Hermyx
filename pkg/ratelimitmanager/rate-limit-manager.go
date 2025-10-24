package ratelimitmanager

import (
	"fmt"
	"hermyx/pkg/models"
	"hermyx/pkg/ratelimit"
	"hermyx/pkg/utils/logger"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// RateLimitManager manages rate limiting with a single backend
type RateLimitManager struct {
	limiter      ratelimit.IRateLimiter
	logger       *logger.Logger
	healthTicker *time.Ticker
	stopChan     chan struct{}
	closeOnce    sync.Once
}

// NewRateLimitManager creates a RateLimitManager configured with the provided limiter and logger.
// It initializes internal channels and starts periodic health monitoring for the limiter.
func NewRateLimitManager(limiter ratelimit.IRateLimiter, logger *logger.Logger) *RateLimitManager {
	manager := &RateLimitManager{
		limiter:  limiter,
		logger:   logger,
		stopChan: make(chan struct{}),
	}

	// Start health monitoring
	manager.startHealthMonitoring()

	return manager
}

// Resolve merges route-specific rate limit config with global config
func (rlm *RateLimitManager) Resolve(globalConfig *models.RateLimitConfig, routeConfig *models.RateLimitConfig) *models.RateLimitConfig {
	return ratelimit.Resolve(globalConfig, routeConfig)
}

// Check checks if a request should be allowed based on the resolved config
func (rlm *RateLimitManager) Check(ctx *fasthttp.RequestCtx, config *models.RateLimitConfig) *ratelimit.RateLimitResult {
	if rlm == nil || rlm.limiter == nil || config == nil || !config.Enabled {
		// Rate limiting is disabled
		if config != nil && !config.Enabled {
			rlm.logger.Debug("Rate limiting is disabled for this request")
		} else if config == nil {
			rlm.logger.Warn("Rate limiting config is nil, allowing request")
		} else if rlm.limiter == nil {
			rlm.logger.Warn("Rate limiter is nil, allowing request")
		}
		return &ratelimit.RateLimitResult{
			Allowed:   true,
			Remaining: -1,
			ResetTime: time.Time{},
			Limit:     -1,
		}
	}

	key := rlm.BuildKey(ctx, config)
	if key == "" {
		// Cannot derive a safe key; allow the request to avoid penalizing all traffic
		rlm.logger.Warn("Cannot derive rate limit key, allowing request to avoid blocking all traffic")
		return &ratelimit.RateLimitResult{
			Allowed:   true,
			Remaining: -1,
			ResetTime: time.Time{},
			Limit:     -1,
		}
	}
	allowed, remaining, resetTime := rlm.limiter.AllowWithLimit(key, *config.Requests, *config.Window)

	if allowed {
		rlm.logger.Debug(fmt.Sprintf("Rate limit check passed for key '%s': %d/%d remaining", key, remaining, *config.Requests))
	} else {
		rlm.logger.Warn(fmt.Sprintf("Rate limit exceeded for key '%s': %d/%d remaining, reset at %v", key, remaining, *config.Requests, resetTime))
	}

	return &ratelimit.RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetTime: resetTime,
		Limit:     *config.Requests,
		Key:       key,
	}
}

// BuildKey builds a rate limit key based on the configuration with warning logs
func (rlm *RateLimitManager) BuildKey(ctx *fasthttp.RequestCtx, config *models.RateLimitConfig) string {
	if config == nil || len(config.KeyBy) == 0 {
		// Default to IP
		rlm.logger.Debug("Using default IP-based rate limiting")
		return rlm.getClientIP(ctx)
	}

	var parts []string
	for _, keyType := range config.KeyBy {
		if keyType == "ip" {
			parts = append(parts, rlm.getClientIP(ctx))
		} else if strings.HasPrefix(keyType, "header:") {
			headerName := strings.TrimPrefix(keyType, "header:")
			headerValue := string(ctx.Request.Header.Peek(headerName))
			if headerValue != "" {
				parts = append(parts, headerValue)
			} else {
				// Required header missing; fall back to IP to avoid empty key
				rlm.logger.Warn(fmt.Sprintf("Required header '%s' missing, falling back to IP for rate limiting", headerName))
				parts = append(parts, rlm.getClientIP(ctx))
			}
		} else {
			// Unknown key type, use as-is
			rlm.logger.Warn(fmt.Sprintf("Unknown key type '%s', using as-is for rate limiting", keyType))
			parts = append(parts, keyType)
		}
	}

	if len(parts) == 0 {
		// Fallback to IP if no valid keys
		rlm.logger.Warn("No valid key parts found, falling back to IP for rate limiting")
		return rlm.getClientIP(ctx)
	}

	key := strings.Join(parts, ":")
	rlm.logger.Debug(fmt.Sprintf("Generated rate limit key: %s", key))
	return key
}

// getClientIP extracts the client IP from the request
func (rlm *RateLimitManager) getClientIP(ctx *fasthttp.RequestCtx) string {
	// Check X-Forwarded-For header first
	xff := string(ctx.Request.Header.Peek("X-Forwarded-For"))
	if xff != "" {
		// Take the first IP in the list
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		rlm.logger.Debug(fmt.Sprintf("Using X-Forwarded-For IP: %s", ip))
		return ip
	}

	// Check X-Real-IP header
	xri := string(ctx.Request.Header.Peek("X-Real-IP"))
	if xri != "" {
		rlm.logger.Debug(fmt.Sprintf("Using X-Real-IP: %s", xri))
		return xri
	}

	// Fall back to RemoteAddr
	ip := ctx.RemoteIP().String()
	rlm.logger.Debug(fmt.Sprintf("Using RemoteAddr IP: %s", ip))
	return ip
}

// Reset resets the rate limit for a specific key
func (rlm *RateLimitManager) Reset(key string) {
	if rlm != nil && rlm.limiter != nil {
		rlm.logger.Debug(fmt.Sprintf("Resetting rate limit for key: %s", key))
		rlm.limiter.Reset(key)
	} else {
		rlm.logger.Warn("Cannot reset rate limit: limiter is nil")
	}
}

// SetHeaders sets the rate limit headers on the response
func (rlm *RateLimitManager) SetHeaders(ctx *fasthttp.RequestCtx, result *ratelimit.RateLimitResult, config *models.RateLimitConfig) {
	if result == nil {
		return
	}

	if config == nil || config.Headers == nil {
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

// startHealthMonitoring starts the background health monitoring goroutine
func (rlm *RateLimitManager) startHealthMonitoring() {
	if rlm.limiter == nil {
		return
	}

	// Check health every 30 seconds
	rlm.healthTicker = time.NewTicker(30 * time.Second)

	go func() {
		for {
			select {
			case <-rlm.healthTicker.C:
				rlm.performHealthCheck()
			case <-rlm.stopChan:
				return
			}
		}
	}()
}

// performHealthCheck performs a health check and logs if unhealthy
func (rlm *RateLimitManager) performHealthCheck() {
	if rlm.limiter == nil {
		return
	}

	err := rlm.limiter.Health()
	if err != nil {
		rlm.logger.Error(fmt.Sprintf("Rate limiter health check failed: %v", err))
	}
}

// Close cleans up resources
func (rlm *RateLimitManager) Close() error {
	var err error
	rlm.closeOnce.Do(func() {
		// Stop health monitoring
		if rlm.healthTicker != nil {
			rlm.healthTicker.Stop()
		}
		if rlm.stopChan != nil {
			close(rlm.stopChan)
			rlm.stopChan = nil
		}
		// Close the limiter
		if rlm.limiter != nil {
			err = rlm.limiter.Close()
		}
	})
	return err
}