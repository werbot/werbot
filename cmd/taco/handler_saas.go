//go:build saas

package main

import (
	"github.com/werbot/werbot/add-on/saas/api/subscription"
	"github.com/werbot/werbot/api"
)

func handler(h *api.Handler) {
	subscription.New(h).Routes()
}
