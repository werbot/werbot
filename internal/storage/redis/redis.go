package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Handler is ...
type Handler interface {
	Close() error
	Ping() error
	Set(key string, value any, expiration time.Duration) error
	Get(key string) *redis.StringCmd
	Delete(key string) (int64, error)

	Client() *redis.Client
}

// rdb is a Redis-backed implementation of the Handler interface.
type rdb struct {
	ctx    context.Context
	client *redis.Client
}

// NewClient creates a new Handler backed by Redis using the given options.
func NewClient(ctx context.Context, client *redis.Client) Handler {
	return &rdb{
		ctx:    ctx,
		client: client,
	}
}

// Close closes the underlying Redis client's connections.
func (r *rdb) Close() error {
	return r.client.Close()
}

// Ping provides a way to ping a Redis server.
func (c rdb) Ping() error {
	_, err := c.client.Ping(c.ctx).Result()
	return err
}

// Set is provides a way to set values in Redis.
func (c rdb) Set(key string, value any, expiration time.Duration) error {
	err := c.client.Set(c.ctx, key, value, expiration).Err()
	return err
}

// Get is provides a way to retrieve values from Redis.
func (c rdb) Get(key string) *redis.StringCmd {
	return c.client.Get(c.ctx, key)
}

// Delete is provides a way to delete values from Redis.
func (c rdb) Delete(key string) (int64, error) {
	return c.client.Del(c.ctx, key).Result()
}

// Client is provides a way to obtain a Redis client.
func (c rdb) Client() *redis.Client {
	return c.client
}
