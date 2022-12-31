package info

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/pkg/logger"
)

type handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *handler {
	log := logger.New("web/info")

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
	h.App.Get("/v1/update", h.Auth, h.getUpdate)
	h.App.Get("/v1/info", h.Auth, h.getInfo)
	h.App.Get("/v1/version", h.Auth, h.getVersion)

	routes(h)
}
