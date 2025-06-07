package models

import "time"

const (
	CACHE_KEY_PATH   = "path"
	CACHE_KEY_METHOD = "method"
	CACHE_KEY_QUERY  = "query"
)

type LogConfig struct {
	ToFile   bool   `yaml:"toFile"`
	FilePath string `yaml:"filePath"`
	ToStdout bool   `yaml:"toStdout"`
	Prefix   string `yaml:"prefix"`
	Flags    int    `yaml:"flags"`
}

type CacheKeyConfig struct {
	Type           []string `yaml:"type"`
	ExcludeMethods []string `yaml:"excludeMethods"`
}

type CacheConfig struct {
	Enabled        bool            `yaml:"enabled"`
	Ttl            time.Duration   `yaml:"ttl"`
	Capacity       uint64          `yaml:"capacity"`
	KeyConfig      *CacheKeyConfig `yaml:"keyConfig"`
	MaxContentSize uint64          `yaml:"maxContentSize"`
}

type ServerConfig struct {
	Port uint16 `yaml:"port"`
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
	Log    *LogConfig    `yaml:"log"`
	Server *ServerConfig `yaml:"server"`
	Cache  *CacheConfig  `yaml:"cache"`
	Routes []RouteConfig `yaml:"routes"`
}
