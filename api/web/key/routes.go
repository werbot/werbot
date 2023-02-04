package key

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *Handler {
	log := logger.New("web/key")

	return &Handler{
		Handler: &web.Handler{
			App:  h.App,
			Grpc: h.Grpc,
			Auth: h.Auth,
		},
		log: log,
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
