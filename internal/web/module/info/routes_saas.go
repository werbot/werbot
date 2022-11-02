//go:build saas

package info

import (
	"github.com/werbot/werbot/internal/web/middleware"
)

func routes(h *Handler, a middleware.Middleware) {
	h.app.Get("/v1/update/version", h.getUpdateVersion)
}
