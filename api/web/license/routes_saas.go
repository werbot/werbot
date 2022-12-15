//go:build saas

package license

func routes(h *handler) {
	h.App.Get("/license/expired", h.getLicenseExpired)
	h.App.Post("/license", h.postLicense)
}
