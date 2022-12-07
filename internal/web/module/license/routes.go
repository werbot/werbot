package license

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
)

// Handler is ...
type Handler struct {
	app       *fiber.App
	grpc      *grpc.ClientService
	auth      fiber.Handler
	publicKey string
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService, auth fiber.Handler, publicKey string) *Handler {
	return &Handler{
		app:       app,
		grpc:      grpc,
		auth:      auth,
		publicKey: publicKey,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.app.Get("/v1/license/info", h.auth, h.getLicenseInfo)

	routes(h)
}
