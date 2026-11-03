//go:build ee && !saas

package main

import (
	"google.golang.org/grpc"

	ee "github.com/werbot/werbot/add-on/ee"
	"github.com/werbot/werbot/internal/addon"
)

// registerAddons registers the gRPC services of every add-on enabled in this
// build.
func registerAddons(s *grpc.Server, deps addon.Deps) {
	ee.New().RegisterGRPC(s, deps)
}
