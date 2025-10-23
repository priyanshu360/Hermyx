package ratelimitmanager

import (
	"fmt"
	"hermyx/pkg/models"
	"hermyx/pkg/ratelimit"
	"hermyx/pkg/utils/logger"
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
		return &ratelimit.RateLimitResult{
			Allowed:   true,
			Remaining: -1,
			ResetTime: time.Time{},
			Limit:     -1,
		}
	}

	key := ratelimit.BuildKey(ctx, config)
	allowed, remaining, resetTime := rlm.limiter.AllowWithLimit(key, config.Requests, config.Window)

	return &ratelimit.RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetTime: resetTime,
		Limit:     config.Requests,
		Key:       key,
	}
}

// Reset resets the rate limit for a specific key
func (rlm *RateLimitManager) Reset(key string) {
	if rlm != nil && rlm.limiter != nil {
		rlm.limiter.Reset(key)
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
		ctx.Response.Header.Set("X-Ratelimit-Limit", fmt.Sprintf("%d", result.Limit))
	}
	if config.Headers.IncludeRemaining {
		ctx.Response.Header.Set("X-Ratelimit-Remaining", fmt.Sprintf("%d", result.Remaining))
	}
	if config.Headers.IncludeReset {
		ctx.Response.Header.Set("X-Ratelimit-Reset", fmt.Sprintf("%d", result.ResetTime.Unix()))
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
