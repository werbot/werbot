//go:build saas

package info

func routes(h *Handler) {
	h.app.Get("/v1/update/version", h.getUpdateVersion)
}
