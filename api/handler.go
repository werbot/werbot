package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/storage/redis"
)

// Handler is ...
type Handler struct {
	App   *fiber.App
	Grpc  *grpc.ClientService
	Redis redis.Handler
	Auth  fiber.Handler
}
