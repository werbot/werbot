package member

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New is ...
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes is ...
func (h *Handler) Routes() {
	// project invite
	h.App.Post("/v1/members/invite/:invite", h.postMembersInviteActivate)

	memberV1 := h.App.Group("/v1/members", h.Auth)

	// Project section
	memberV1.Get("/", h.getProjectMember)
	memberV1.Post("/", h.addProjectMember)
	memberV1.Patch("/", h.updateProjectMember)
	memberV1.Delete("/", h.deleteProjectMember)

	memberV1.Patch("/active", h.updateProjectMemberStatus)
	memberV1.Get("/search", h.getUsersWithoutProject)

	memberV1.Get("/invite", h.getProjectMembersInvite)
	memberV1.Post("/invite", h.addProjectMemberInvite)
	memberV1.Delete("/invite", h.deleteProjectMemberInvite)

	// Server section
	memberV1.Get("/server", h.getServerMember)
	memberV1.Post("/server", h.addServerMember)
	memberV1.Patch("/server", h.updateServerMember)
	memberV1.Delete("/server", h.deleteServerMember)

	memberV1.Patch("/server/active", h.updateServerMemberActive)
	memberV1.Get("/server/search", h.getMembersWithoutServer)
}
