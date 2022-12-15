//go:build !saas

package license

import (
	"github.com/werbot/werbot/internal/web/middleware"
)

func routes(h *Handler) {
	// h.app.Get("/v1/license", h.auth, h.getLicenseInfo)
}
