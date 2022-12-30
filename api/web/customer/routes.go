package customer

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal/logger"
)

type handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *handler {
	log := logger.New("web/customer")

	return &handler{
		Handler: &web.Handler{
			App:  h.App,
			Grpc: h.Grpc,
			Auth: h.Auth,
		},
		log: log,
	}
}

// Routes is ...
func (h *handler) Routes() {
	customerV1 := h.App.Group("/v1/customers", h.Auth)
	customerV1.Get("/", h.getCustomer)
	customerV1.Delete("/", h.deleteCustomer)
}
