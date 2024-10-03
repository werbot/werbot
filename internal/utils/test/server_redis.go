package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/werbot/werbot/pkg/storage/redis"
	"github.com/werbot/werbot/pkg/worker"
	"github.com/werbot/werbot/pkg/worker/asynq"
)

// RedisService encapsulates a Redis connection for testing.
type RedisService struct {
	conn   *redis.Connect
	worker worker.Client
	test   *testing.T
}

// ServerRedis initializes a new RedisService with a miniredis instance.
func ServerRedis(ctx context.Context, t *testing.T) *RedisService {
	s := miniredis.RunT(t)

	newRedis := redis.New(ctx, &redis.Config{Addr: s.Addr()})
	newWorker, err := asynq.NewClient(fmt.Sprintf("redis://%s/1", s.Addr()))
	if err != nil {
		t.Fatalf("Server asynq exited with error: %v", err)
	}

	return &RedisService{
		conn:   newRedis,
		worker: newWorker,
		test:   t,
	}
}

// Close terminates the Redis connection and handles any errors.
func (d *RedisService) Close() {
	if err := d.conn.Client.Close(); err != nil {
		d.test.Error(err)
	}

	if err := d.worker.Close(); err != nil {
		d.test.Error(err)
	}
}
