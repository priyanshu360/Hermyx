package models

import "time"

const (
	CACHE_KEY_PATH   = "path"
	CACHE_KEY_METHOD = "method"
	CACHE_KEY_QUERY  = "query"
	CACHE_KEY_HEADER = "header"
)

const (
	CACHE_TYPE_MEMORY = "memory"
	CACHE_TYPE_DISK   = "disk"
	CACHE_TYPE_REDIS  = "redis"
)

type LogConfig struct {
	ToFile       bool   `yaml:"toFile"`
	FilePath     string `yaml:"filePath"`
	ToStdout     bool   `yaml:"toStdout"`
	Prefix       string `yaml:"prefix"`
	Flags        int    `yaml:"flags"`
	DebugEnabled bool   `yaml:"debugEnabled"`
}

type HeaderCacheKeyConfig struct {
	Key string `yaml:"key"`
}

type CacheKeyConfig struct {
	Type           []string                `yaml:"type"`
	ExcludeMethods []string                `yaml:"excludeMethods"`
	Headers        []*HeaderCacheKeyConfig `yaml:"headers"`
}

type RedisConfig struct {
	Address      string        `yaml:"address"`
	Password     string        `yaml:"password"`
	DB           *int          `yaml:"db"`
	DefaultTTL   time.Duration `yaml:"defaultTtl"`
	KeyNamespace string        `yaml:"namespace"`
	FailOpen     *bool         `yaml:"failOpen"` // If true, allow requests when Redis is down; if false, block them
}

type CacheConfig struct {
	Type           string          `yaml:"type"`
	Enabled        bool            `yaml:"enabled"`
	Ttl            time.Duration   `yaml:"ttl"`
	Capacity       uint64          `yaml:"capacity"`
	KeyConfig      *CacheKeyConfig `yaml:"keyConfig"`
	MaxContentSize uint64          `yaml:"maxContentSize"`
	Redis          *RedisConfig    `yaml:"redis"`
}

type ServerConfig struct {
	Port uint16 `yaml:"port"`
}

type StorageConfig struct {
	Path string `yaml:"path"`
}

type RateLimitConfig struct {
	Enabled       bool                    `yaml:"enabled"`
	Requests      *int64                  `yaml:"requests"`
	Window        *time.Duration          `yaml:"window"`
	Storage       string                  `yaml:"storage"`
	KeyBy         []string                `yaml:"keyBy"`
	BlockDuration *time.Duration          `yaml:"blockDuration"`
	StatusCode    *int                    `yaml:"statusCode"`
	Message       string                  `yaml:"message"`
	Redis         *RedisConfig            `yaml:"redis"`
	Headers       *RateLimitHeadersConfig `yaml:"headers"`
}

type RateLimitHeadersConfig struct {
	IncludeRemaining bool `yaml:"includeRemaining"`
	IncludeReset     bool `yaml:"includeReset"`
	IncludeLimit     bool `yaml:"includeLimit"`
}

type RouteConfig struct {
	Name      string           `yaml:"name"`
	Path      string           `yaml:"path"`
	Target    string           `yaml:"target"`
	Include   []string         `yaml:"include"`
	Exclude   []string         `yaml:"exclude"`
	Cache     *CacheConfig     `yaml:"cache"`
	RateLimit *RateLimitConfig `yaml:"rateLimit"`
}

type HermyxConfig struct {
	Log       *LogConfig       `yaml:"log"`
	Server    *ServerConfig    `yaml:"server"`
	Cache     *CacheConfig     `yaml:"cache"`
	Storage   *StorageConfig   `yaml:"storage"`
	RateLimit *RateLimitConfig `yaml:"rateLimit"`
	Routes    []RouteConfig    `yaml:"routes"`
}
