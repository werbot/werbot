package test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/werbot/werbot/internal/storage/cache"
)

// NewCache is ...
func NewCache(t *testing.T) cache.Cache {
	s := miniredis.RunT(t)
	return cache.New(context.TODO(), &redis.Options{
		Addr: s.Addr(),
	})
}
