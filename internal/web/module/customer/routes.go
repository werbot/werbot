package customer

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
	customerV1 := h.app.Group("/v1/customers", h.auth)
	customerV1.Get("/", h.getCustomer)
	customerV1.Delete("/", h.deleteCustomer)
}
