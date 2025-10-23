package ratelimit

import (
	"fmt"
	"hermyx/pkg/models"
	"hermyx/pkg/utils/logger"
	"sync"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

// createTestLogger creates a logger for testing
func createTestLogger(t *testing.T) *logger.Logger {
	logger, err := logger.NewLogger(&models.LogConfig{
		DebugEnabled: true,
		ToStdout:     false, // Disable stdout for tests
		ToFile:       false,
		FilePath:     "",
		Prefix:       "[Test]",
		Flags:        0,
	})
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}
	return logger
}

// createBenchmarkLogger creates a logger for benchmarking
func createBenchmarkLogger(b *testing.B) *logger.Logger {
	logger, err := logger.NewLogger(&models.LogConfig{
		DebugEnabled: false, // Disable debug for benchmarks
		ToStdout:     false,
		ToFile:       false,
		FilePath:     "",
		Prefix:       "[Benchmark]",
		Flags:        0,
	})
	if err != nil {
		b.Fatalf("Failed to create benchmark logger: %v", err)
	}
	return logger
}

// ==========================================
// Constructor Tests
// ==========================================

func TestNewRateLimiter_Memory(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
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

	limiter, err := NewRateLimiter(config, createTestLogger(t))
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

	_, err := NewRateLimiter(config, createTestLogger(t))
	if err == nil {
		t.Error("Should error for invalid storage type")
	}
}

// ==========================================
// Basic Allow Tests
// ==========================================

func TestRateLimiter_Allow_Basic(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 5,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key := BuildKey(ctx, config)

	// Should allow first 5 requests
	for i := 0; i < 5; i++ {
		allowed, remaining, _ := limiter.Allow(key)
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
		expectedRemaining := int64(4 - i)
		if remaining != expectedRemaining {
			t.Errorf("Request %d: expected remaining=%d, got %d", i+1, expectedRemaining, remaining)
		}
	}

	// 6th request should be blocked
	allowed, remaining, _ := limiter.Allow(key)
	if allowed {
		t.Error("6th request should be blocked")
	}
	if remaining != 0 {
		t.Errorf("Expected 0 remaining, got %d", remaining)
	}
}

func TestRateLimiter_Allow_ZeroLimit(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 0, // No requests allowed
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key := BuildKey(ctx, config)

	// First request should be blocked
	allowed, _, _ := limiter.Allow(key)
	if allowed {
		t.Error("Request should be blocked when limit is 0")
	}
}

func TestRateLimiter_Allow_HighLimit(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 10000,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key := BuildKey(ctx, config)

	// Should allow many requests
	for i := 0; i < 100; i++ {
		allowed, _, _ := limiter.Allow(key)
		if !allowed {
			t.Errorf("Request %d should be allowed with high limit", i+1)
		}
	}
}

// ==========================================
// Token Refill Tests
// ==========================================

func TestRateLimiter_TokenRefill(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 5,
		Window:   5 * time.Second, // 1 token per second
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key := BuildKey(ctx, config)

	// Consume all tokens
	for i := 0; i < 5; i++ {
		allowed, _, _ := limiter.Allow(key)
		if !allowed {
			t.Fatalf("Request %d should be allowed", i+1)
		}
	}

	// Next should be blocked
	allowed, _, _ := limiter.Allow(key)
	if allowed {
		t.Error("Should be blocked after consuming all tokens")
	}

	// Wait for 2 tokens to refill (2 seconds)
	time.Sleep(2100 * time.Millisecond)

	// Should allow 2 more requests
	for i := 0; i < 2; i++ {
		allowed, _, _ := limiter.Allow(key)
		if !allowed {
			t.Errorf("Request %d should be allowed after refill", i+1)
		}
	}

	// Should be blocked again
	allowed, _, _ = limiter.Allow(key)
	if allowed {
		t.Error("Should be blocked again after consuming refilled tokens")
	}
}

func TestRateLimiter_FullRefill(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 3,
		Window:   3 * time.Second,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key := BuildKey(ctx, config)

	// Consume all tokens
	for i := 0; i < 3; i++ {
		limiter.Allow(key)
	}

	// Wait for full refill
	time.Sleep(3100 * time.Millisecond)

	// Should have all 3 tokens back
	for i := 0; i < 3; i++ {
		allowed, remaining, _ := limiter.Allow(key)
		if !allowed {
			t.Errorf("Request %d should be allowed after full refill", i+1)
		}
		if i == 0 && remaining != 2 {
			t.Errorf("After full refill and first request, expected 2 remaining, got %d", remaining)
		}
	}
}

// ==========================================
// Key Building Tests
// ==========================================

func TestRateLimiter_BuildKey_IP(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.100")

	key := BuildKey(ctx, config)
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

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-API-Key", "my-api-key-123")

	key := BuildKey(ctx, config)
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

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	ctx.Request.Header.Set("X-User-ID", "user123")

	key := BuildKey(ctx, config)
	expected := "192.168.1.1:user123"
	if key != expected {
		t.Errorf("Expected key '%s', got '%s'", expected, key)
	}
}

func TestRateLimiter_BuildKey_MultipleHeaders(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"header:X-API-Key", "header:X-App-ID"},
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-API-Key", "key123")
	ctx.Request.Header.Set("X-App-ID", "app456")

	key := BuildKey(ctx, config)
	expected := "key123:app456"
	if key != expected {
		t.Errorf("Expected key '%s', got '%s'", expected, key)
	}
}

func TestRateLimiter_BuildKey_MissingHeader(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"header:X-API-Key"},
	}

	ctx := &fasthttp.RequestCtx{}
	// Don't set the header

	key := BuildKey(ctx, config)
	if key != "" {
		t.Errorf("Expected empty key when header missing, got '%s'", key)
	}
}

func TestRateLimiter_BuildKey_EmptyKeyBy(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{},
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")

	key := BuildKey(ctx, config)
	// Should default to IP when KeyBy is empty
	if key != "192.168.1.1" {
		t.Errorf("Expected default IP key, got '%s'", key)
	}
}

// ==========================================
// IP Resolution Tests
// ==========================================

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

func TestGetClientIP_XForwardedForPriority(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "203.0.113.1")
	ctx.Request.Header.Set("X-Real-IP", "198.51.100.5")

	ip := getClientIP(ctx)
	// X-Forwarded-For should take priority
	if ip != "203.0.113.1" {
		t.Errorf("Expected X-Forwarded-For IP '203.0.113.1', got '%s'", ip)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	// No headers set - should fall back to RemoteAddr
	// Note: In tests, RemoteAddr might be empty or have a default value
	ip := getClientIP(ctx)
	// Just verify it doesn't panic and returns something
	if ip == "" {
		t.Log("RemoteAddr fallback returned empty string (expected in test context)")
	}
}

// ==========================================
// Isolation Tests (Different Keys)
// ==========================================

func TestRateLimiter_IsolationByIP(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 3,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	// User 1
	ctx1 := &fasthttp.RequestCtx{}
	ctx1.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key1 := BuildKey(ctx1, config)

	// User 2
	ctx2 := &fasthttp.RequestCtx{}
	ctx2.Request.Header.Set("X-Forwarded-For", "192.168.1.2")
	key2 := BuildKey(ctx2, config)

	// User 1 consumes all tokens
	for i := 0; i < 3; i++ {
		limiter.Allow(key1)
	}
	allowed1, _, _ := limiter.Allow(key1)
	if allowed1 {
		t.Error("User 1 should be blocked")
	}

	// User 2 should still have tokens
	allowed2, remaining2, _ := limiter.Allow(key2)
	if !allowed2 {
		t.Error("User 2 should not be blocked")
	}
	if remaining2 != 2 {
		t.Errorf("User 2 should have 2 remaining, got %d", remaining2)
	}
}

func TestRateLimiter_IsolationByAPIKey(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 2,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"header:X-API-Key"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	// API Key 1
	ctx1 := &fasthttp.RequestCtx{}
	ctx1.Request.Header.Set("X-API-Key", "key-aaa")
	key1 := BuildKey(ctx1, config)
	if key1 != "key-aaa" {
		t.Errorf("Expected key 'key-aaa', got '%s'", key1)
	}

	// API Key 2
	ctx2 := &fasthttp.RequestCtx{}
	ctx2.Request.Header.Set("X-API-Key", "key-bbb")
	key2 := BuildKey(ctx2, config)
	if key2 != "key-bbb" {
		t.Errorf("Expected key 'key-bbb', got '%s'", key2)
	}

	// Key 1 uses both tokens
	limiter.Allow(key1)
	limiter.Allow(key1)
	allowed1, _, _ := limiter.Allow(key1)
	if allowed1 {
		t.Error("API Key 1 should be blocked")
	}

	// Key 2 should have full quota
	allowed2, remaining2, _ := limiter.Allow(key2)
	if !allowed2 {
		t.Error("API Key 2 should not be blocked")
	}
	if remaining2 != 1 {
		t.Errorf("API Key 2 should have 1 remaining, got %d", remaining2)
	}
}

// ==========================================
// Concurrency Tests
// ==========================================

func TestRateLimiter_Concurrent(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key := BuildKey(ctx, config)

	var wg sync.WaitGroup
	successCount := int64(0)
	blockedCount := int64(0)
	var mu sync.Mutex

	// Launch 200 concurrent requests
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, _ := limiter.Allow(key)
			mu.Lock()
			if allowed {
				successCount++
			} else {
				blockedCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Should allow exactly 100 requests
	if successCount != 100 {
		t.Errorf("Expected 100 successful requests, got %d", successCount)
	}
	if blockedCount != 100 {
		t.Errorf("Expected 100 blocked requests, got %d", blockedCount)
	}
}

func TestRateLimiter_ConcurrentDifferentKeys(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 50,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	var wg sync.WaitGroup

	// 10 different IPs, each making 60 requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(ip int) {
			defer wg.Done()
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.Header.Set("X-Forwarded-For", fmt.Sprintf("192.168.1.%d", ip))
			key := BuildKey(ctx, config)

			successCount := 0
			for j := 0; j < 60; j++ {
				allowed, _, _ := limiter.Allow(key)
				if allowed {
					successCount++
				}
			}

			// Each IP should get exactly 50 requests
			if successCount != 50 {
				t.Errorf("IP %d: expected 50 successful requests, got %d", ip, successCount)
			}
		}(i)
	}

	wg.Wait()
}

// ==========================================
// Resolve Tests (Config Inheritance)
// ==========================================

func TestResolve_BasicOverride(t *testing.T) {
	global := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 1000,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
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

func TestResolve_HeadersInheritance(t *testing.T) {
	global := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 1000,
		Window:   1 * time.Minute,
		Headers: &models.RateLimitHeadersConfig{
			IncludeLimit:     true,
			IncludeRemaining: true,
			IncludeReset:     true,
		},
	}

	route := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
	}

	resolved := Resolve(global, route)
	if resolved.Headers == nil {
		t.Fatal("Headers should be inherited from global")
	}
	if !resolved.Headers.IncludeLimit || !resolved.Headers.IncludeRemaining || !resolved.Headers.IncludeReset {
		t.Error("All header flags should be inherited as true from global")
	}
}

func TestResolve_HeadersOverride(t *testing.T) {
	global := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 1000,
		Window:   1 * time.Minute,
		Headers: &models.RateLimitHeadersConfig{
			IncludeLimit:     true,
			IncludeRemaining: true,
			IncludeReset:     true,
		},
	}

	route := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Headers: &models.RateLimitHeadersConfig{
			IncludeLimit:     true,
			IncludeRemaining: false,
			IncludeReset:     true,
		},
	}

	resolved := Resolve(global, route)
	if resolved.Headers == nil {
		t.Fatal("Headers should be set")
	}
	if !resolved.Headers.IncludeLimit {
		t.Error("IncludeLimit should be true")
	}
	if resolved.Headers.IncludeRemaining {
		t.Error("IncludeRemaining should be false (route override)")
	}
	if !resolved.Headers.IncludeReset {
		t.Error("IncludeReset should be true")
	}
}

func TestResolve_NoGlobalHeaders(t *testing.T) {
	global := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 1000,
	}

	route := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
	}

	resolved := Resolve(global, route)
	if resolved.Headers != nil {
		t.Error("Headers should be nil when neither global nor route specifies them")
	}
}

func TestResolve_NilRoute(t *testing.T) {
	global := &models.RateLimitConfig{
		Enabled:       true,
		Requests:      1000,
		Window:        1 * time.Minute,
		Storage:       "memory",
		KeyBy:         []string{"ip"},
		BlockDuration: 5 * time.Minute,
	}

	resolved := Resolve(global, nil)

	// Should return global config unchanged
	if resolved.Requests != 1000 {
		t.Errorf("Expected requests 1000, got %d", resolved.Requests)
	}
	if resolved.Window != 1*time.Minute {
		t.Errorf("Expected window 1m, got %v", resolved.Window)
	}
}

// ==========================================
// Reset Tests
// ==========================================

func TestRateLimiter_Reset(t *testing.T) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 3,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key := BuildKey(ctx, config)

	// Consume all tokens
	for i := 0; i < 3; i++ {
		limiter.Allow(key)
	}

	// Should be blocked
	allowed, _, _ := limiter.Allow(key)
	if allowed {
		t.Error("Should be blocked after consuming all tokens")
	}

	// Reset the key
	limiter.Reset(key)

	// Should now allow requests again
	allowed, remaining, _ := limiter.Allow(key)
	if !allowed {
		t.Error("Should be allowed after reset")
	}
	if remaining != 2 {
		t.Errorf("Expected 2 remaining after reset and one request, got %d", remaining)
	}
}

// ==========================================
// Benchmark Tests
// ==========================================

func BenchmarkRateLimiter_Allow(b *testing.B) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 1000000,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, _ := NewRateLimiter(config, createBenchmarkLogger(b))
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key := BuildKey(ctx, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow(key)
	}
}

func BenchmarkRateLimiter_BuildKey_IP(b *testing.B) {
	config := &models.RateLimitConfig{
		KeyBy: []string{"ip"},
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildKey(ctx, config)
	}
}

func BenchmarkRateLimiter_BuildKey_Combined(b *testing.B) {
	config := &models.RateLimitConfig{
		KeyBy: []string{"ip", "header:X-API-Key", "header:X-User-ID"},
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	ctx.Request.Header.Set("X-API-Key", "key123")
	ctx.Request.Header.Set("X-User-ID", "user456")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildKey(ctx, config)
	}
}

func BenchmarkRateLimiter_Concurrent(b *testing.B) {
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 1000000,
		Window:   1 * time.Minute,
		Storage:  "memory",
		KeyBy:    []string{"ip"},
	}

	limiter, _ := NewRateLimiter(config, createBenchmarkLogger(b))
	defer limiter.Close()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "192.168.1.1")
	key := BuildKey(ctx, config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow(key)
		}
	})
}

// ==========================================
// Redis Health Monitoring Tests
// ==========================================

func TestRedisRateLimiter_HealthMonitoring(t *testing.T) {
	// This test requires a Redis instance to be running
	// Skip if Redis is not available
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "redis",
		Redis: &models.RedisConfig{
			Address: "localhost:6379",
			DB:      &[]int{0}[0],
		},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Skip("Redis not available, skipping health monitoring test")
	}
	defer limiter.Close()

	// Test that we can get a Redis limiter
	redisLimiter, ok := limiter.(*RedisRateLimiter)
	if !ok {
		t.Fatal("Expected RedisRateLimiter")
	}

	// Test initial health status
	// Note: IsHealthy and GetHealthStatus methods are not implemented
	// if !redisLimiter.IsHealthy() {
	//	t.Error("Redis should be healthy initially")
	// }

	// Test health status details
	// healthStatus := redisLimiter.GetHealthStatus()
	// if healthStatus["healthy"] != true {
	//	t.Error("Health status should show healthy")
	// }
	// if healthStatus["fail_open"] != true {
	//	t.Error("Should default to fail open mode")
	// }

	// Test ping functionality
	err = redisLimiter.Ping()
	if err != nil {
		t.Errorf("Ping should succeed: %v", err)
	}
}

func TestRedisRateLimiter_FailOpenMode(t *testing.T) {
	// Test with fail open mode (default)
	config := &models.RateLimitConfig{
		Enabled:  true,
		Requests: 100,
		Window:   1 * time.Minute,
		Storage:  "redis",
		Redis: &models.RedisConfig{
			Address:  "localhost:6379",
			DB:       &[]int{0}[0],
			FailOpen: &[]bool{true}[0],
		},
	}

	limiter, err := NewRateLimiter(config, createTestLogger(t))
	if err != nil {
		t.Skip("Redis not available, skipping fail open test")
	}
	defer limiter.Close()

	redisLimiter, ok := limiter.(*RedisRateLimiter)
	if !ok {
		t.Fatal("Expected RedisRateLimiter")
	}

	// Test fail open mode
	if !redisLimiter.failOpen {
		t.Error("Should be in fail open mode")
	}

	// Test changing fail open mode
	// Note: SetFailOpenMode method is not implemented
	// redisLimiter.SetFailOpenMode(false)
	// if redisLimiter.failOpen {
	//	t.Error("Should be in fail closed mode after change")
	// }

	// redisLimiter.SetFailOpenMode(true)
	// if !redisLimiter.failOpen {
	//	t.Error("Should be back in fail open mode")
	// }
}
