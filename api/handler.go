package api

import (
	"google.golang.org/grpc"

	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/internal/storage/redis"
)

// Handler is ...
type Handler struct {
	App   *fiber.App
	Grpc  *grpc.ClientConn
	Redis redis.Handler
	Auth  fiber.Handler
}
