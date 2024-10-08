package user

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles user-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new user handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the user-related routes.
func (h *Handler) Routes() {
	apiV1 := h.App.Group("/v1/users", h.Auth)
	apiV1.Get("/list", h.users)
	apiV1.Get("/", h.user)
	apiV1.Post("/", h.addUser)
	apiV1.Patch("/", h.updateUser)
	apiV1.Patch("/password", h.updatePassword)
	apiV1.Post("/delete", h.deleteUser)
	apiV1.Delete("/delete/:token<guid>", h.deleteUser)
}
