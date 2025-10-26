package ratelimitmanager

import (
	"hermyx/pkg/models"
	"hermyx/pkg/ratelimit"
	"hermyx/pkg/utils/logger"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

func createTestLogger(t *testing.T) *logger.Logger {
	logger, err := logger.NewLogger(&models.LogConfig{
		DebugEnabled: true,
		ToStdout:     false,
		ToFile:       false,
		Prefix:       "[Test]",
		Flags:        0,
	})
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	return logger
}

func int64Ptr(v int64) *int64 {
	return &v
}

func durationPtr(v time.Duration) *time.Duration {
	return &v
}

func TestNewRateLimitManager(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
	if manager.limiter == nil {
		t.Error("Manager limiter should not be nil")
	}
}

func TestNewRateLimitManager_NilLimiter(t *testing.T) {
	manager := NewRateLimitManager(nil, createTestLogger(t))
	defer manager.Close()
	if manager == nil {
		t.Fatal("Manager should not be nil even with nil limiter")
	}
	ctx := &fasthttp.RequestCtx{}
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
	}
	result := manager.Check(ctx, config)
	if !result.Allowed {
		t.Error("Should allow when limiter is nil")
	}
}

func TestRateLimitManager_Check_Disabled(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	ctx := &fasthttp.RequestCtx{}
	checkConfig := &models.RateLimitConfig{
		Enabled: false,
	}
	result := manager.Check(ctx, checkConfig)
	if !result.Allowed {
		t.Error("Should allow when rate limiting is disabled")
	}
	if result.Limit != -1 {
		t.Errorf("Expected limit=-1 for disabled, got %d", result.Limit)
	}
}

func TestRateLimitManager_Check_NilConfig(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	ctx := &fasthttp.RequestCtx{}
	result := manager.Check(ctx, nil)
	if !result.Allowed {
		t.Error("Should allow when config is nil")
	}
}

func TestRateLimitManager_Check_Basic(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(5),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	for i := 0; i < 5; i++ {
		result := manager.Check(ctx, config)
		if !result.Allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
		if result.Limit != 5 {
			t.Errorf("Expected limit=5, got %d", result.Limit)
		}
	}
	result := manager.Check(ctx, config)
	if result.Allowed {
		t.Error("6th request should be blocked")
	}
	if result.Remaining != 0 {
		t.Errorf("Expected remaining=0, got %d", result.Remaining)
	}
}

func TestRateLimitManager_BuildKey_IP(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "203.0.113.50")
	checkConfig := &models.RateLimitConfig{
		KeyBy: []string{"ip"},
	}
	key := manager.BuildKey(ctx, checkConfig)
	if key != "203.0.113.50" {
		t.Errorf("Expected IP key, got '%s'", key)
	}
}

func TestRateLimitManager_BuildKey_Header(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-API-Key", "secret-key-123")
	checkConfig := &models.RateLimitConfig{
		KeyBy: []string{"header:X-API-Key"},
	}
	key := manager.BuildKey(ctx, checkConfig)
	if key != "secret-key-123" {
		t.Errorf("Expected API key, got '%s'", key)
	}
}

func TestRateLimitManager_BuildKey_MissingHeader(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	checkConfig := &models.RateLimitConfig{
		KeyBy: []string{"header:X-Missing-Header"},
	}
	key := manager.BuildKey(ctx, checkConfig)
	if key != "192.168.1.1" {
		t.Errorf("Expected fallback to IP, got '%s'", key)
	}
}

func TestRateLimitManager_BuildKey_Combined(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	ctx.Request.Header.Set("X-User-ID", "user123")
	checkConfig := &models.RateLimitConfig{
		KeyBy: []string{"ip", "header:X-User-ID"},
	}
	key := manager.BuildKey(ctx, checkConfig)
	expected := "192.168.1.1:user123"
	if key != expected {
		t.Errorf("Expected combined key '%s', got '%s'", expected, key)
	}
}

func TestRateLimitManager_SetHeaders(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	result := &ratelimit.RateLimitResult{
		Allowed:   true,
		Remaining: 95,
		ResetTime: time.Now().Add(time.Minute),
		Limit:     100,
	}
	config := &models.RateLimitConfig{
		Headers: &models.RateLimitHeadersConfig{
			IncludeLimit:     true,
			IncludeRemaining: true,
			IncludeReset:     true,
		},
	}
	limiter, _ := ratelimit.NewRateLimiter(&models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	manager.SetHeaders(ctx, result, config)
	limitHeader := string(ctx.Response.Header.Peek("X-RateLimit-Limit"))
	remainingHeader := string(ctx.Response.Header.Peek("X-RateLimit-Remaining"))
	resetHeader := string(ctx.Response.Header.Peek("X-RateLimit-Reset"))
	if limitHeader != "100" {
		t.Errorf("Expected X-RateLimit-Limit=100, got '%s'", limitHeader)
	}
	if remainingHeader != "95" {
		t.Errorf("Expected X-RateLimit-Remaining=95, got '%s'", remainingHeader)
	}
	if resetHeader == "" {
		t.Error("Expected X-RateLimit-Reset to be set")
	}
}

func TestRateLimitManager_SetHeaders_Selective(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	result := &ratelimit.RateLimitResult{
		Allowed:   true,
		Remaining: 50,
		ResetTime: time.Now().Add(time.Minute),
		Limit:     100,
	}
	config := &models.RateLimitConfig{
		Headers: &models.RateLimitHeadersConfig{
			IncludeLimit:     true,
			IncludeRemaining: false,
			IncludeReset:     true,
		},
	}
	limiter, _ := ratelimit.NewRateLimiter(&models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	manager.SetHeaders(ctx, result, config)
	limitHeader := string(ctx.Response.Header.Peek("X-RateLimit-Limit"))
	remainingHeader := string(ctx.Response.Header.Peek("X-RateLimit-Remaining"))
	if limitHeader != "100" {
		t.Errorf("Expected X-RateLimit-Limit=100, got '%s'", limitHeader)
	}
	if remainingHeader != "" {
		t.Error("X-RateLimit-Remaining should not be set when disabled")
	}
}

func TestRateLimitManager_Reset(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(3),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	for i := 0; i < 3; i++ {
		manager.Check(ctx, config)
	}
	result := manager.Check(ctx, config)
	if result.Allowed {
		t.Error("Should be blocked after limit")
	}
	manager.Reset(result.Key)
	result = manager.Check(ctx, config)
	if !result.Allowed {
		t.Error("Should be allowed after reset")
	}
}

func TestRateLimitManager_Close_Multiple(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
	}
	limiter, _ := ratelimit.NewRateLimiter(config, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	err := manager.Close()
	if err != nil {
		t.Errorf("First close should not error: %v", err)
	}
	err = manager.Close()
	if err != nil {
		t.Errorf("Second close should not error: %v", err)
	}
}

func TestRateLimitManager_Resolve(t *testing.T) {
	global := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(1000),
		Window:   durationPtr(time.Minute),
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}
	route := &models.RateLimitConfig{
		Enabled:  true,
		Requests: int64Ptr(100),
	}
	limiter, _ := ratelimit.NewRateLimiter(global, createTestLogger(t))
	manager := NewRateLimitManager(limiter, createTestLogger(t))
	defer manager.Close()
	resolved := manager.Resolve(global, route)
	if *resolved.Requests != 100 {
		t.Errorf("Expected requests=100, got %d", *resolved.Requests)
	}
	if resolved.Storage != "memory" {
		t.Errorf("Expected storage=memory, got %s", resolved.Storage)
	}
}
