package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Connect struct {
	Client *redis.Client
	Ctx    context.Context
}

type RedisConfig struct {
	Addr     string
	Password string
}

func New(ctx context.Context, conf *RedisConfig) *Connect {
	client := redis.NewClient(&redis.Options{
		Addr:     conf.Addr,
		Password: conf.Password,
	})

	return &Connect{
		Client: client,
		Ctx:    ctx,
	}
}
