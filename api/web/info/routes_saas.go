//go:build saas

package info

func routes(h *handler) {
	h.App.Get("/v1/update/version", h.getUpdateVersion)
}
