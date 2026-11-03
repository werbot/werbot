//go:build !saas && !ee

package main

import (
	"github.com/werbot/werbot/api"
)

// registerAddons mounts optional add-on routes. The core edition ships no
// add-ons.
func registerAddons(_ *api.Handler) {}
