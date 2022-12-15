package user

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
	log := logger.New("module/user")

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
	userV1 := h.App.Group("/v1/users", h.Auth)
	userV1.Get("/", h.getUser)
	userV1.Post("/", h.addUser)
	userV1.Patch("/", h.patchUser)
	userV1.Delete("/", h.deleteUser)

	userV1.Patch("/password", h.patchPassword)
}
