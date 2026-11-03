//go:build saas

package main

import (
	"google.golang.org/grpc"

	"github.com/werbot/werbot/add-on/saas"
	"github.com/werbot/werbot/internal/addon"
)

// registerAddons registers the gRPC services of every add-on enabled in this
// build.
func registerAddons(s *grpc.Server, deps addon.Deps) {
	saas.New().RegisterGRPC(s, deps)
}
