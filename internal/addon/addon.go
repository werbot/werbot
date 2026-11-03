// Package addon defines the contract every optional add-on implements so the
// command binaries can register HTTP routes and gRPC services without knowing
// which edition is being built.
package addon

import (
	"google.golang.org/grpc"

	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/storage/redis"
	"github.com/werbot/werbot/pkg/worker"
)

// Deps holds the infrastructure an add-on may need when it registers gRPC
// services on the server side.
type Deps struct {
	DB     *postgres.Connect
	Redis  *redis.Connect
	Worker worker.Client
}

// Addon is a self-contained feature module built on top of the core API.
type Addon interface {
	// Name returns the add-on identifier, matching its folder in add-on/.
	Name() string
	// Routes mounts the add-on HTTP routes on the given handler.
	Routes(*api.Handler)
	// RegisterGRPC registers the add-on gRPC services on the given server.
	RegisterGRPC(grpc.ServiceRegistrar, Deps)
}
