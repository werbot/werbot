package license

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	*api.Handler
	publicKey string
	log       logger.Logger
}

// New is ...
func New(h *api.Handler, publicKey string) *Handler {
	return &Handler{
		Handler:   h,
		log:       logger.New(),
		publicKey: publicKey,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.App.Get("/v1/license/info", h.Auth, h.getLicenseInfo)
}
