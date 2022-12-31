package server

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/logger"
)

type handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *handler {
	log := logger.New("web/server")

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
	keyMiddleware := middleware.Key(h.Grpc)

	// public
	serviceV1 := h.App.Group("/v1/service", keyMiddleware.Execute())
	serviceV1.Post("/server", h.addServiceServer)
	serviceV1.Patch("/status", h.patchServiceServerStatus)

	// private
	serverV1 := h.App.Group("/v1/servers", h.Auth)
	serverV1.Patch("/active", h.patchServerStatus)

	serverV1.Get("/", h.getServer)
	serverV1.Post("/", h.addServer)
	serverV1.Patch("/", h.patchServer)
	serverV1.Delete("/", h.deleteServer)

	serverV1.Get("/activity", h.getServerActivity)
	serverV1.Patch("/activity", h.patchServerActivity)

	serverV1.Get("/firewall", h.getServerFirewall)
	serverV1.Post("/firewall", h.postServerFirewall)
	serverV1.Delete("/firewall", h.deleteServerFirewall)
	serverV1.Patch("/firewall", h.patchAccessPolicy)

	serverV1.Get("/share", h.getServersShareForUser)
	serverV1.Post("/share", h.postServersShareForUser)
	serverV1.Patch("/share", h.patchServerShareForUser)
	serverV1.Delete("/share", h.deleteServerShareForUser)

	serverV1.Get("/access", h.getServerAccess)

	serverV1.Get("/name", h.serverNameByID)
}
