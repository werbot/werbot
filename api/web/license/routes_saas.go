//go:build saas

package license

func routes(h *Handler) {
	h.App.Get("/license/expired", h.getLicenseExpired)
	h.App.Post("/license", h.postLicense)
}
