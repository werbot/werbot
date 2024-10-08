package key

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles key-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new key handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the key-related routes.
func (h *Handler) Routes() {
	apiV1 := h.App.Group("/v1/keys", h.Auth)
	apiV1.Get("/generate", h.generateNewKey)

	apiV1.Get("/", h.keys)
	apiV1.Post("/", h.addKey)

	apiV1.Get("/:key_id<guid>", h.key)
	apiV1.Patch("/:key_id<guid>", h.updateKey)
	apiV1.Delete("/:key_id<guid>", h.deleteKey)
}
