package license

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	*web.Handler
	publicKey string
	log       logger.Logger
}

// New is ...
func New(h *web.Handler, publicKey string) *Handler {
	log := logger.New("web/license")

	return &Handler{
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
func (h *Handler) Routes() {
	h.App.Get("/v1/license/info", h.Auth, h.getLicenseInfo)

	routes(h)
}
