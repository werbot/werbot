package customer

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *Handler {
	log := logger.New("web/customer")

	return &Handler{
		Handler: &web.Handler{
			App:  h.App,
			Grpc: h.Grpc,
			Auth: h.Auth,
		},
		log: log,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	customerV1 := h.App.Group("/v1/customers", h.Auth)
	customerV1.Get("/", h.getCustomer)
	customerV1.Delete("/", h.deleteCustomer)
}
