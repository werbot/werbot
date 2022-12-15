//go:build !saas

package license

import (
	"github.com/werbot/werbot/internal/web/middleware"
)

func routes(h *handler) {
	// h.App.Get("/v1/license", h.Auth, h.getLicenseInfo)
}
