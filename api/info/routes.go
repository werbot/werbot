package info

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler represents an instance of info handler.
type Handler struct {
	// Handler embeds api.Handler to provide access to methods like App, Grpc, Auth etc.
	*api.Handler

	// log stores an instance of Logger to log messages.
	log logger.Logger
}

// New returns a new instance of Handler.
func New(h *api.Handler) *Handler {
	log := logger.New()

	return &Handler{
		Handler: &api.Handler{
			App:  h.App,
			Grpc: h.Grpc,
			Auth: h.Auth,
		},
		log: log,
	}
}

// Routes sets routes for Handler.
func (infoHandler *Handler) Routes() {
	infoHandler.App.Get("/v1/update", infoHandler.Auth, infoHandler.getUpdate)
	infoHandler.App.Get("/v1/info", infoHandler.Auth, infoHandler.getInfo)
	infoHandler.App.Get("/v1/version", infoHandler.Auth, infoHandler.getVersion)
}
