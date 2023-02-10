package test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	rdb "github.com/werbot/werbot/internal/storage/redis"
)

// NewCache is ...
func NewCache(t *testing.T) rdb.Handler {
	s := miniredis.RunT(t)
	return rdb.NewClient(context.TODO(), &redis.Options{
		Addr: s.Addr(),
	})
}
