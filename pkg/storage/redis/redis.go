package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Connect  is ...
type Connect struct {
	Client *redis.Client
	Ctx    context.Context
}

// Config is ...
type Config struct {
	Addr     string
	Password string
}

// New  is ...
func New(ctx context.Context, conf *Config) *Connect {
	client := redis.NewClient(&redis.Options{
		Addr:     conf.Addr,
		Password: conf.Password,
		// DB:       0,
	})

	return &Connect{
		Client: client,
		Ctx:    ctx,
	}
}
