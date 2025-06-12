package cachemanager

import (
	"hermyx/pkg/models"
	"sort"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

type ICache interface {
	Set(key string, value []byte, ttl time.Duration) error
	Get(key string) ([]byte, bool, error)
	Delete(key string)
	Close() error
}

type CacheManager struct {
	cache ICache
}

func NewCacheManager(cache ICache) *CacheManager {
	return &CacheManager{cache: cache}
}

func (cm *CacheManager) Resolve(engineConfig *models.CacheConfig, routeConfig *models.CacheConfig) *models.CacheConfig {
	if routeConfig == nil {
		return engineConfig
	}

	if !routeConfig.Enabled {
		return routeConfig
	}

	config := routeConfig

	if config.Ttl == 0 && engineConfig != nil {
		config.Ttl = engineConfig.Ttl
	}

	if config.KeyConfig == nil && engineConfig != nil {
		config.KeyConfig = engineConfig.KeyConfig
	}

	if config.MaxContentSize == 0 && engineConfig != nil {
		config.MaxContentSize = engineConfig.MaxContentSize
	}

	sort.Strings(config.KeyConfig.Type)

	return config
}

func (cm *CacheManager) Set(key string, value []byte, ttl time.Duration) {
	cm.cache.Set(key, value, ttl)
}

func (cm *CacheManager) Get(key string) ([]byte, bool, error) {
	return cm.cache.Get(key)
}

func (cm *CacheManager) GetKey(cacheKeyConfig *models.CacheKeyConfig, ctx *fasthttp.RequestCtx) string {
	var keyParts []string

	for _, keyType := range cacheKeyConfig.Type {
		switch keyType {
		case models.CACHE_KEY_METHOD:
			keyParts = append(keyParts, strings.ToLower(string(ctx.Method())))
		case models.CACHE_KEY_PATH:
			keyParts = append(keyParts, string(ctx.Path()))
		case models.CACHE_KEY_QUERY:
			keyParts = append(keyParts, string(ctx.QueryArgs().QueryString()))
		}
	}

	return strings.Join(keyParts, "|")
}

func (cm *CacheManager) Delete(key string) {
	cm.cache.Delete(key)
}

func (cm *CacheManager) Close() error {
	return cm.cache.Close()
}
