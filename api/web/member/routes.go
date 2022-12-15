package member

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
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
	// project invite
	h.app.Post("/v1/members/invite/:invite", h.postProjectMembersInviteActivate)

	memberV1 := h.app.Group("/v1/members", h.auth)

	// Project section
	memberV1.Get("/", h.getProjectMember)
	memberV1.Post("/", h.addProjectMember)
	memberV1.Patch("/", h.patchProjectMember)
	memberV1.Delete("/", h.deleteProjectMember)

	memberV1.Patch("/active", h.patchProjectMemberStatus)
	memberV1.Get("/search", h.getUsersWithoutProject)

	memberV1.Get("/invite", h.getProjectMembersInvite)
	memberV1.Post("/invite", h.addProjectMemberInvite)
	memberV1.Delete("/invite", h.deleteProjectMemberInvite)

	// Server section
	memberV1.Get("/server", h.getServerMember)
	memberV1.Post("/server", h.addServerMember)
	memberV1.Patch("/server", h.patchServerMember)
	memberV1.Delete("/server", h.deleteServerMember)

	memberV1.Patch("/server/active", h.patchServerMemberStatus)
	memberV1.Get("/server/search", h.getMembersWithoutServer)
}
