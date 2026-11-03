//go:build !saas

package main

import (
	"google.golang.org/grpc"

	"github.com/werbot/werbot/internal/addon"
)

// registerAddons registers optional add-on gRPC services. The core edition
// ships no add-ons.
func registerAddons(_ *grpc.Server, _ addon.Deps) {}
