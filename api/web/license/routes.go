package license

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal/logger"
)

type handler struct {
	*web.Handler
	publicKey string
	log       logger.Logger
}

// New is ...
func New(h *web.Handler, publicKey string) *handler {
	log := logger.New("web/license")

	return &handler{
		Handler: &web.Handler{
			App:  h.App,
			Grpc: h.Grpc,
			Auth: h.Auth,
		},
		log:       log,
		publicKey: publicKey,
	}
}

// Routes is ...
func (h *handler) Routes() {
	h.App.Get("/v1/license/info", h.Auth, h.getLicenseInfo)

	routes(h)
}
