//go:build !saas

package license

import (
	"github.com/werbot/werbot/internal/web/middleware"
)

func routes(h *Handler, a middleware.Middleware) {
	// h.app.Get("/v1/license", a.Execute(), h.getLicenseInfo)
}
