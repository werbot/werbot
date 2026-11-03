//go:build saas

package main

import (
	"github.com/werbot/werbot/add-on/saas"
	"github.com/werbot/werbot/api"
)

// registerAddons mounts the routes of every add-on enabled in this build.
func registerAddons(h *api.Handler) {
	saas.New().Routes(h)
}
