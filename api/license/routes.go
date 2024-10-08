package license

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles license-related routes.
type Handler struct {
	*api.Handler
	publicKey string
	log       logger.Logger
}

// New creates a new license handler.
func New(h *api.Handler, publicKey string) *Handler {
	return &Handler{
		Handler:   h,
		log:       logger.New(),
		publicKey: publicKey,
	}
}

// Routes sets up the license-related routes.
func (h *Handler) Routes() {
	apiV1 := h.App.Group("/v1/license", h.Auth)
	apiV1.Get("/info", h.licenseInfo)
}
