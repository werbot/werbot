package test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	rdb "github.com/werbot/werbot/internal/storage/redis"
)

// RedisService is ...
type RedisService struct {
	rdb.Handler
	test *testing.T
}

// Redis is ...
func Redis(ctx context.Context, t *testing.T) *RedisService {
	s := miniredis.RunT(t)
	return &RedisService{
		Handler: rdb.NewClient(ctx, redis.NewClient(&redis.Options{
			Addr: s.Addr(),
		})),
		test: t,
	}
}

// Close is ...
func (d *RedisService) Close() {
	if err := d.Handler.Close(); err != nil {
		d.test.Error(err)
	}
}
