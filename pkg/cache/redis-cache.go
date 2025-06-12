package cache

import (
	"context"
	"hermyx/pkg/models"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client     *redis.Client
	namespace  string
	defaultTTL time.Duration
	ctx        context.Context
}

func NewRedisCache(config *models.RedisConfig) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       *config.DB,
	})

	return &RedisCache{
		client:     client,
		namespace:  config.KeyNamespace,
		defaultTTL: config.DefaultTTL,
		ctx:        context.Background(),
	}
}

func (r *RedisCache) Set(key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = r.defaultTTL
	}
	return r.client.Set(r.ctx, r.key(key), value, ttl).Err()
}

func (r *RedisCache) Get(key string) ([]byte, bool, error) {
	val, err := r.client.Get(r.ctx, r.key(key)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (r *RedisCache) Delete(key string) {
	r.client.Del(r.ctx, r.key(key))
}

func (r *RedisCache) Len() int {
	keys, err := r.client.Keys(r.ctx, r.namespace+"*").Result()
	if err != nil {
		return 0
	}
	return len(keys)
}

func (r *RedisCache) key(k string) string {
	return r.namespace + k
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}

