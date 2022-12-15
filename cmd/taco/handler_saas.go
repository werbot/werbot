//go:build saas

package main

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/api/web/subscription"
)

func handler(h *web.Handler) {
	subscription.New(h).Routes()
}
