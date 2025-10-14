package ratelimit

import (
	"hermyx/pkg/models"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

func TestNewRateLimiter_Memory(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
	}

	limiter, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create memory rate limiter: %v", err)
	}
	defer limiter.Close()

	if limiter == nil {
		t.Error("Limiter should not be nil")
	}
}

func TestNewRateLimiter_Disabled(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled: false,
	}

	limiter, err := NewRateLimiter(config)
	if err != nil {
		t.Errorf("Should not error for disabled limiter: %v", err)
	}
	if limiter != nil {
		t.Error("Limiter should be nil when disabled")
	}
}

func TestNewRateLimiter_InvalidStorage(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "invalid-storage",
	}

	_, err := NewRateLimiter(config)
	if err == nil {
		t.Error("Should error for invalid storage type")
	}
}

func TestRateLimiter_Check(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 5,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	// Create a mock request context
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")

	// Should allow first 5 requests
	for i := 0; i < 5; i++ {
		result := limiter.Check(ctx)
		if !result.Allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should be blocked
	result := limiter.Check(ctx)
	if result.Allowed {
		t.Error("6th request should be blocked")
	}
	if result.Remaining != 0 {
		t.Errorf("Expected 0 remaining, got %d", result.Remaining)
	}
}

func TestRateLimiter_BuildKey_IP(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.100")

	key := limiter.buildKey(ctx)
	if key != "192.168.1.100" {
		t.Errorf("Expected key '192.168.1.100', got '%s'", key)
	}
}

func TestRateLimiter_BuildKey_Header(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"header:X-API-Key"},
	}

	limiter, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-API-Key", "my-api-key-123")

	key := limiter.buildKey(ctx)
	if key != "my-api-key-123" {
		t.Errorf("Expected key 'my-api-key-123', got '%s'", key)
	}
}

func TestRateLimiter_BuildKey_Combined(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip", "header:X-User-ID"},
	}

	limiter, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	ctx.Request.Header.Set("X-User-ID", "user123")

	key := limiter.buildKey(ctx)
	expected := "192.168.1.1:user123"
	if key != expected {
		t.Errorf("Expected key '%s', got '%s'", expected, key)
	}
}

func TestResolve(t *testing.T) {
	global := &models.RateLimitConfig{
		Enabled:       true,
		Requests:      1000,
		Window:        1 * time.Minute,
		Storage:       "memory",
		KeyBy:         []string{"ip"},
		BlockDuration: 5 * time.Minute,
		StatusCode:    429,
		Message:       "Global rate limit exceeded",
	}

	route := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
	}

	resolved := Resolve(global, route)

	if resolved.Requests != 100 {
		t.Errorf("Expected requests 100, got %d", resolved.Requests)
	}
	if resolved.Window != 1*time.Minute {
		t.Errorf("Expected window 1m, got %v", resolved.Window)
	}
	// Should inherit from global
	if resolved.Storage != "memory" {
		t.Errorf("Expected storage 'memory', got '%s'", resolved.Storage)
	}
	if len(resolved.KeyBy) != 1 || resolved.KeyBy[0] != "ip" {
		t.Errorf("Expected keyBy ['ip'], got %v", resolved.KeyBy)
	}
}

func TestResolve_Disabled(t *testing.T) {
	global := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 1000,
	}

	route := &models.RateLimitConfig{
		Enabled: false,
	}

	resolved := Resolve(global, route)

	if resolved.Enabled {
		t.Error("Resolved config should be disabled")
	}
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")

	ip := getClientIP(ctx)
	if ip != "203.0.113.1" {
		t.Errorf("Expected IP '203.0.113.1', got '%s'", ip)
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Real-IP", "198.51.100.5")

	ip := getClientIP(ctx)
	if ip != "198.51.100.5" {
		t.Errorf("Expected IP '198.51.100.5', got '%s'", ip)
	}
}

func TestSetRateLimitHeaders(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	result := &RateLimitResult{
		Allowed:   true,
		Remaining: 95,
		ResetTime: time.Now().Add(1 * time.Minute),
		Limit:     100,
	}
	config := &models.RateLimitConfig{
		Headers: &models.RateLimitHeadersConfig{
			IncludeLimit:     true,
			IncludeRemaining: true,
			IncludeReset:     true,
		},
	}

	SetRateLimitHeaders(ctx, result, config)

	limitHeader := string(ctx.Response.Header.Peek("X-RateLimit-Limit"))
	if limitHeader != "100" {
		t.Errorf("Expected limit header '100', got '%s'", limitHeader)
	}

	remainingHeader := string(ctx.Response.Header.Peek("X-RateLimit-Remaining"))
	if remainingHeader != "95" {
		t.Errorf("Expected remaining header '95', got '%s'", remainingHeader)
	}

	resetHeader := string(ctx.Response.Header.Peek("X-RateLimit-Reset"))
	if resetHeader == "" {
		t.Error("Reset header should be set")
	}
}

func BenchmarkRateLimiter_Check(b *testing.B) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100000,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, _ := NewRateLimiter(config)
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Check(ctx)
	}
}

