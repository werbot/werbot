//go:build saas

package info

func routes(h *Handler) {
	h.App.Get("/v1/update/version", h.getUpdateVersion)
}
