package key

import (
	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/internal/grpc"
)

// Handler is ...
type Handler struct {
	app  *fiber.App
	grpc *grpc.ClientService
	auth fiber.Handler
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService, auth fiber.Handler) *Handler {
	return &Handler{
		app:  app,
		grpc: grpc,
		auth: auth,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	keyV1 := h.app.Group("/v1/keys", h.auth)
	keyV1.Get("/generate", h.getGenerateNewKey)

	keyV1.Get("/", h.getKey)
	keyV1.Post("/", h.addKey)
	keyV1.Patch("/", h.patchKey)
	keyV1.Delete("/", h.deleteKey)
}
