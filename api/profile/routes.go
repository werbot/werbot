package profile

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles profile-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new profile handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the profile-related routes.
func (h *Handler) Routes() {
	apiV1 := h.App.Group("/v1/profiles", h.Auth)
	apiV1.Get("/list", h.profiles)
	apiV1.Get("/", h.profile)
	apiV1.Post("/", h.addProfile)
	apiV1.Patch("/", h.updateProfile)
	apiV1.Patch("/password", h.updatePassword)
	apiV1.Post("/delete", h.deleteProfile)
	apiV1.Delete("/delete/:token<guid>", h.deleteProfile)
}
