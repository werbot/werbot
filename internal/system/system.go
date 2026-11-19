// Package system implements SystemService.
package system

import (
	"context"
	"net/url"
	"strings"

	"connectrpc.com/connect"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/version"
)

// Service implements werbotv1connect.SystemServiceHandler.
type Service struct{ geoIPAttribution bool }

// New returns a Service for the GeoIP database at geoIPURL (WERBOT_GEOIP_URL).
func New(geoIPURL string) *Service { return &Service{geoIPAttribution: dbIPLite(geoIPURL)} }

// dbIPLite reports whether u is DB-IP Lite: a /free/ path on db-ip.com or a
// subdomain. Its license requires the UI to credit DB-IP.
func dbIPLite(u string) bool {
	p, err := url.Parse(strings.Replace(u, "%s", "2006-01", 1))
	if err != nil {
		return false
	}
	h := p.Hostname()
	return (h == "db-ip.com" || strings.HasSuffix(h, ".db-ip.com")) && strings.Contains(p.Path, "/free/")
}

// GetVersion returns build metadata and whether the UI credits DB-IP.
func (s *Service) GetVersion(_ context.Context, _ *connect.Request[werbotv1.GetVersionRequest]) (*connect.Response[werbotv1.GetVersionResponse], error) {
	return connect.NewResponse(&werbotv1.GetVersionResponse{
		Version:          version.Version,
		Commit:           version.Commit,
		BuildDate:        version.BuildDate,
		GeoipAttribution: s.geoIPAttribution,
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
