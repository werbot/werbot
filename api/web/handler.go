package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/storage/cache"
)

// Handler is ...
type Handler struct {
	App   *fiber.App
	Grpc  *grpc.ClientService
	Cache cache.Cache
	Auth  fiber.Handler
}
