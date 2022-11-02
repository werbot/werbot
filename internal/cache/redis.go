package cache

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type redisCache struct {
	ctx    context.Context
	client *redis.Client
}

// NewRedisClient is ...
func NewRedisClient(ctx context.Context, opts *redis.Options) Cache {
	return &redisCache{
		ctx:    ctx,
		client: redis.NewClient(opts),
	}
}

// Ping is ...
func (c redisCache) Ping() error {
	_, err := c.client.Ping(c.ctx).Result()
	return err
}

// Set is ...
func (c redisCache) Set(key string, value any, expiration time.Duration) error {
	return c.client.Set(c.ctx, key, value, expiration).Err()
}

// Get is ...
func (c redisCache) Get(key string) (string, error) {
	return c.client.Get(c.ctx, key).Result()
}

// Delete is ...
func (c redisCache) Delete(key string) (int64, error) {
	return c.client.Del(c.ctx, key).Result()
}
