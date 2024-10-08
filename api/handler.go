package api

import (
	"google.golang.org/grpc"

	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/pkg/storage/redis"
)

// Handler is ...
type Handler struct {
	App     *fiber.App
	Grpc    *grpc.ClientConn
	Redis   *redis.Connect
	Auth    fiber.Handler
	EnvMode string
}
