package server

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/web/middleware"
)

// Handler is ...
type Handler struct {
	app  *fiber.App
	grpc *grpc.ClientService
	auth fiber.Handler
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService, auth fiber.Handler) *Handler {
	return &Handler{
		app:  app,
		grpc: grpc,
		auth: auth,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	keyMiddleware := middleware.Key(h.grpc)

	// public
	serviceV1 := h.app.Group("/v1/service", keyMiddleware.Execute())
	serviceV1.Post("/server", h.addServiceServer)
	serviceV1.Patch("/status", h.patchServiceServerStatus)

	// private
	serverV1 := h.app.Group("/v1/servers", h.auth)
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
}
