package member

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal/logger"
)

type handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *handler {
	log := logger.New("module/member")

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
	// project invite
	h.App.Post("/v1/members/invite/:invite", h.postProjectMembersInviteActivate)

	memberV1 := h.App.Group("/v1/members", h.Auth)

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
