// Package system implements SystemService.
package system

import (
	"context"

	"connectrpc.com/connect"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/version"
)

// Service implements werbotv1connect.SystemServiceHandler.
type Service struct{}

// New returns a Service.
func New() *Service { return &Service{} }

// GetVersion returns build metadata.
func (*Service) GetVersion(_ context.Context, _ *connect.Request[werbotv1.GetVersionRequest]) (*connect.Response[werbotv1.GetVersionResponse], error) {
	return connect.NewResponse(&werbotv1.GetVersionResponse{
		Version:   version.Version,
		Commit:    version.Commit,
		BuildDate: version.BuildDate,
	}), nil
}
