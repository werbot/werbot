// Package system implements SystemService.
package system

import (
	"context"

	"connectrpc.com/connect"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/clientip"
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

// ListCountries returns the ISO 3166-1 country list.
func (*Service) ListCountries(_ context.Context, _ *connect.Request[werbotv1.ListCountriesRequest]) (*connect.Response[werbotv1.ListCountriesResponse], error) {
	return connect.NewResponse(&werbotv1.ListCountriesResponse{Countries: countries()}), nil
}

// GetClientIP returns the caller address resolved by clientip.Middleware, or "" when invalid.
func (*Service) GetClientIP(ctx context.Context, _ *connect.Request[werbotv1.GetClientIPRequest]) (*connect.Response[werbotv1.GetClientIPResponse], error) {
	ip := ""
	if a := clientip.From(ctx); a.IsValid() {
		ip = a.String()
	}
	return connect.NewResponse(&werbotv1.GetClientIPResponse{Ip: ip}), nil
}
