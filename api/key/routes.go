package key

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
	keyV1 := h.App.Group("/v1/keys", h.Auth)
	keyV1.Get("/generate", h.getGenerateNewKey)

	keyV1.Get("/", h.getKey)
	keyV1.Post("/", h.addKey)
	keyV1.Patch("/", h.updateKey)
	keyV1.Delete("/", h.deleteKey)
}
