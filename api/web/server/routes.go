package server

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *Handler {
	log := logger.New("web/server")

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
	keyMiddleware := middleware.Key(h.Grpc)

	// public
	serviceV1 := h.App.Group("/v1/service", keyMiddleware.Execute())
	serviceV1.Post("/server", h.addServiceServer)
	serviceV1.Patch("/status", h.updateServiceServerStatus)

	// private
	serverV1 := h.App.Group("/v1/servers", h.Auth)
	serverV1.Patch("/active", h.updateServerStatus)

	serverV1.Get("/", h.server)
	serverV1.Post("/", h.addServer)
	serverV1.Patch("/", h.updateServer)
	serverV1.Delete("/", h.deleteServer)

	serverV1.Get("/activity", h.serverActivity)
	serverV1.Patch("/activity", h.updateServerActivity)

	serverV1.Get("/firewall", h.serverFirewall)
	serverV1.Post("/firewall", h.addServerFirewall)
	serverV1.Delete("/firewall", h.deleteServerFirewall)
	serverV1.Patch("/firewall", h.updateServerFirewall)

	serverV1.Get("/share", h.serversShareForUser)
	serverV1.Post("/share", h.addServersShareForUser)
	serverV1.Patch("/share", h.updateServerShareForUser)
	serverV1.Delete("/share", h.deleteServerShareForUser)

	serverV1.Get("/access", h.serverAccess)

	serverV1.Get("/name", h.serverNameByID)
}
