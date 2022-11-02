//go:build saas

package license

import (
	"github.com/werbot/werbot/internal/web/middleware"
)

func routes(h *Handler, a middleware.Middleware) {
	h.app.Get("/license/expired", h.getLicenseExpired)
	h.app.Post("/license", h.postLicense)
}
