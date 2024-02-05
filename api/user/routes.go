package user

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
	userV1 := h.App.Group("/v1/users", h.Auth)
	userV1.Get("/", h.getUser)
	userV1.Post("/", h.addUser)
	userV1.Patch("/", h.updateUser)
	userV1.Delete("/", h.deleteUser)

	userV1.Patch("/password", h.updatePassword)
}
