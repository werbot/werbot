//go:build ee && !saas

package main

import (
	ee "github.com/werbot/werbot/add-on/ee"
	"github.com/werbot/werbot/api"
)

// registerAddons mounts the routes of every add-on enabled in this build.
func registerAddons(h *api.Handler) {
	ee.New().Routes(h)
}
