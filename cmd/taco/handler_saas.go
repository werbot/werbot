//go:build saas

package main

import (
	"github.com/werbot/werbot/add-on/saas/api/web/subscription"
	"github.com/werbot/werbot/api/web"
)

func handler(h *web.Handler) {
	subscription.New(h).Routes()
}
