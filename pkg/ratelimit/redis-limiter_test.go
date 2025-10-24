package ratelimit

import (
	"hermyx/pkg/models"
	"testing"
	"time"
)

func TestNewRedisRateLimiter_DefaultValues(t *testing.T) {
	config := &models.RedisConfig{
		Address:      "localhost:6379",
		Password:     "",
		KeyNamespace: "test:",
	}
	limiter := NewRedisRateLimiter(config, 100, time.Minute, createTestLogger(t))
	defer limiter.Close()
	if limiter == nil {
		t.Fatal("Limiter should not be nil")
	}
	if limiter.maxTokens != 100 {
		t.Errorf("Expected maxTokens=100, got %d", limiter.maxTokens)
	}
	if limiter.namespace != "test:" {
		t.Errorf("Expected namespace='test:', got '%s'", limiter.namespace)
	}
	if !limiter.failOpen {
		t.Error("Expected failOpen=true by default")
	}
}

func TestNewRedisRateLimiter_NamespaceFormatting(t *testing.T) {
	config := &models.RedisConfig{
		Address:      "localhost:6379",
		KeyNamespace: "hermyx",
	}
	limiter := NewRedisRateLimiter(config, 100, time.Minute, createTestLogger(t))
	defer limiter.Close()
	if limiter.namespace != "hermyx:" {
		t.Errorf("Expected namespace to be appended with ':', got '%s'", limiter.namespace)
	}
}

func TestNewRedisRateLimiter_EmptyNamespace(t *testing.T) {
	config := &models.RedisConfig{
		Address: "localhost:6379",
	}
	limiter := NewRedisRateLimiter(config, 100, time.Minute, createTestLogger(t))
	defer limiter.Close()
	if limiter.namespace != "hermyx:ratelimit:" {
		t.Errorf("Expected default namespace, got '%s'", limiter.namespace)
	}
}

func TestNewRedisRateLimiter_FailOpenConfig(t *testing.T) {
	failOpen := false
	config := &models.RedisConfig{
		Address:  "localhost:6379",
		FailOpen: &failOpen,
	}
	limiter := NewRedisRateLimiter(config, 100, time.Minute, createTestLogger(t))
	defer limiter.Close()
	if limiter.failOpen {
		t.Error("Expected failOpen=false when explicitly set")
	}
}

func TestRedisRateLimiter_Key(t *testing.T) {
	config := &models.RedisConfig{
		Address:      "localhost:6379",
		KeyNamespace: "hermyx:",
	}
	limiter := NewRedisRateLimiter(config, 100, time.Minute, createTestLogger(t))
	defer limiter.Close()
	key := limiter.key("192.168.1.1")
	expected := "hermyx:192.168.1.1"
	if key != expected {
		t.Errorf("Expected key='%s', got '%s'", expected, key)
	}
}

func TestSafeConvertToInt64_VariousTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
		ok       bool
	}{
		{"int64", int64(42), 42, true},
		{"int", int(42), 42, true},
		{"int32", int32(42), 42, true},
		{"float64", float64(42.0), 42, true},
		{"string valid", "42", 42, true},
		{"string invalid", "abc", 0, false},
		{"bytes valid", []byte("42"), 42, true},
		{"nil", nil, 0, false},
		{"unknown type", struct{}{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := safeConvertToInt64(tt.input)
			if ok != tt.ok {
				t.Errorf("Expected ok=%v, got %v", tt.ok, ok)
			}
			if ok && result != tt.expected {
				t.Errorf("Expected result=%d, got %d", tt.expected, result)
			}
		})
	}
}

func TestSafeConvertToBool_VariousTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
		ok       bool
	}{
		{"bool true", true, true, true},
		{"bool false", false, false, true},
		{"int64 1", int64(1), true, true},
		{"int64 0", int64(0), false, true},
		{"int 1", int(1), true, true},
		{"float64 1", float64(1.0), true, true},
		{"string true", "true", true, true},
		{"string false", "false", false, true},
		{"string 1", "1", true, true},
		{"bytes true", []byte("true"), true, true},
		{"nil", nil, false, false},
		{"unknown", struct{}{}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := safeConvertToBool(tt.input)
			if ok != tt.ok {
				t.Errorf("Expected ok=%v, got %v", tt.ok, ok)
			}
			if ok && result != tt.expected {
				t.Errorf("Expected result=%v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRedisRateLimiter_AllowWithLimit_ZeroLimit(t *testing.T) {
	config := &models.RedisConfig{
		Address: "localhost:6379",
	}
	limiter := NewRedisRateLimiter(config, 100, time.Minute, createTestLogger(t))
	defer limiter.Close()
	allowed, remaining, _ := limiter.AllowWithLimit("test-key", 0, time.Minute)
	if allowed {
		t.Error("Should not allow with zero limit")
	}
	if remaining != 0 {
		t.Errorf("Expected remaining=0, got %d", remaining)
	}
}
