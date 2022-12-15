//go:build saas

package license

func routes(h *Handler) {
	h.app.Get("/license/expired", h.getLicenseExpired)
	h.app.Post("/license", h.postLicense)
}
