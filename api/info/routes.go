package info

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler represents an instance of info handler.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New returns a new instance of Handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets routes for Handler.
func (infoHandler *Handler) Routes() {
	infoHandler.App.Get("/v1/update", infoHandler.Auth, infoHandler.getUpdate)
	infoHandler.App.Get("/v1/info", infoHandler.Auth, infoHandler.getInfo)
	infoHandler.App.Get("/v1/version", infoHandler.Auth, infoHandler.getVersion)
}
