package models

import "time"

const (
	CACHE_KEY_PATH   = "path"
	CACHE_KEY_METHOD = "method"
	CACHE_KEY_QUERY  = "query"
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

type CacheKeyConfig struct {
	Type           []string `yaml:"type"`
	ExcludeMethods []string `yaml:"excludeMethods"`
}

type RedisConfig struct {
	Address      string        `yaml:"address"`
	Password     string        `yaml:"password"`
	DB           *int          `yaml:"db"`
	DefaultTTL   time.Duration `yaml:"defaultTtl"`
	KeyNamespace string        `yaml:"namespace"`
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

type RouteConfig struct {
	Name    string       `yaml:"name"`
	Path    string       `yaml:"path"`
	Target  string       `yaml:"target"`
	Include []string     `yaml:"include"`
	Exclude []string     `yaml:"exclude"`
	Cache   *CacheConfig `yaml:"cache"`
}

type HermyxConfig struct {
	Log     *LogConfig     `yaml:"log"`
	Server  *ServerConfig  `yaml:"server"`
	Cache   *CacheConfig   `yaml:"cache"`
	Storage *StorageConfig `yaml:"storage"`
	Routes  []RouteConfig  `yaml:"routes"`
}
