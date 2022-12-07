package user

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
)

type handler struct {
	app  *fiber.App
	grpc *grpc.ClientService
	auth fiber.Handler
	log  zerolog.Logger
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService, auth fiber.Handler) *handler {
	return &handler{
		app:  app,
		grpc: grpc,
		auth: auth,
		log:  logger.New("module/user"),
	}
}

// Routes is ...
func (h *handler) Routes() {
	userV1 := h.app.Group("/v1/users", h.auth)
	userV1.Get("/", h.getUser)
	userV1.Post("/", h.addUser)
	userV1.Patch("/", h.patchUser)
	userV1.Delete("/", h.deleteUser)

	userV1.Patch("/password", h.patchPassword)
}
