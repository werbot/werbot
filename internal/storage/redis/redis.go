package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Handler is ...
type Handler interface {
	Ping() error
	Set(key string, value any, expiration time.Duration) error
	Get(key string) *redis.StringCmd
	Delete(key string) (int64, error)

	Client() *redis.Client
}

type rdb struct {
	ctx    context.Context
	client *redis.Client
}

// NewClient is ...
func NewClient(ctx context.Context, opts *redis.Options) Handler {
	return &rdb{
		ctx:    ctx,
		client: redis.NewClient(opts),
	}
}

// Ping is ...
func (c rdb) Ping() error {
	_, err := c.client.Ping(c.ctx).Result()
	return err
}

// Set is ...
func (c rdb) Set(key string, value any, expiration time.Duration) error {
	return c.client.Set(c.ctx, key, value, expiration).Err()
}

// Get is ...
func (c rdb) Get(key string) *redis.StringCmd {
	return c.client.Get(c.ctx, key)
}

// Delete is ...
func (c rdb) Delete(key string) (int64, error) {
	return c.client.Del(c.ctx, key).Result()
}

// Client is ...
func (c rdb) Client() *redis.Client {
	return c.client
}
