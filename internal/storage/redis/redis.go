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
	PingCtx(ctx context.Context) error
	Set(key string, value any, expiration time.Duration) error
	SetCtx(ctx context.Context, key string, value any, expiration time.Duration) error
	Get(key string) *redis.StringCmd
	GetCtx(ctx context.Context, key string) *redis.StringCmd
	Delete(key string) (int64, error)
	DeleteCtx(ctx context.Context, key string) (int64, error)

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
	return c.PingCtx(c.ctx)
}

// PingCtx is ...
func (c rdb) PingCtx(ctx context.Context) error {
	_, err := c.client.Ping(ctx).Result()
	return err
}

// Set is provides a way to set values in Redis.
func (c rdb) Set(key string, value any, expiration time.Duration) error {
	return c.SetCtx(c.ctx, key, value, expiration)
}

// SetCtx is ...
func (c rdb) SetCtx(ctx context.Context, key string, value any, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Get is provides a way to retrieve values from Redis.
func (c rdb) Get(key string) *redis.StringCmd {
	return c.GetCtx(c.ctx, key)
}

// GetCtx is ...
func (c rdb) GetCtx(ctx context.Context, key string) *redis.StringCmd {
	return c.client.Get(ctx, key)
}

// Delete is provides a way to delete values from Redis.
func (c rdb) Delete(key string) (int64, error) {
	return c.DeleteCtx(c.ctx, key)
}

// DeleteCtx is ...
func (c rdb) DeleteCtx(ctx context.Context, key string) (int64, error) {
	return c.client.Del(ctx, key).Result()
}

// Client is provides a way to obtain a Redis client.
func (c rdb) Client() *redis.Client {
	return c.client
}
