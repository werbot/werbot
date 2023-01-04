//go:build saas

package subscription

func routes(h *Handler) {

	apiV1 := h.App.Group("/v1/subscriptions", h.Auth)
	apiV1.Get("/", h.getSubscriptions)

	apiV1.Patch("/:subscription_id", h.patchSubscription)
	apiV1.Delete("/:subscription_id", h.deleteSubscription)

	// apiV1.Post("/:subscription_id/stop", h.stopSubscription)
	// apiV1.Post("/:subscription_id/user", h.addSubscriptionToUser)

	apiV1.Get("/plans", h.getSubscriptionPlans)
	apiV1.Get("/plans/:plan_id", h.getSubscriptionPlan)
	apiV1.Patch("/plans/:plan_id", h.patchSubscriptionPlan)
}
