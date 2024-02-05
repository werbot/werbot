package test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/werbot/werbot/internal/storage/redis"
)

// RedisService is ...
type RedisService struct {
	conn *redis.Connect
	test *testing.T
}

// Redis is ...
func Redis(ctx context.Context, t *testing.T) *RedisService {
	s := miniredis.RunT(t)
	return &RedisService{
		conn: redis.New(ctx, &redis.RedisConfig{
			Addr: s.Addr(),
		}),
		test: t,
	}
}

// Close is ...
func (d *RedisService) Close() {
	if err := d.conn.Client.Close(); err != nil {
		d.test.Error(err)
	}
}
