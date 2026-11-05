package firewall_test

import (
	"context"
	"os"
	"testing"

	firewallmessage "github.com/werbot/werbot/internal/core/firewall/proto/message"
	firewallrpc "github.com/werbot/werbot/internal/core/firewall/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"google.golang.org/grpc/codes"
)

// setupFirewall creates the global firewall list owned by the test.
func setupFirewall(t *testing.T) (*test.GRPCHandler, func(t *testing.T)) {
	t.Helper()

	setup, teardownTestCase := test.GRPC(t)
	builder := factory.New(t, setup.Postgres.Conn().Conn)

	listID := builder.FirewallList(factory.FirewallListParams{Name: "global"})
	builder.FirewallCountry(listID, "RU")
	builder.FirewallCountry(listID, "BY")
	builder.FirewallNetwork(listID, "178.239.2.11/32")

	return setup, teardownTestCase
}

func Test_IPAccess(t *testing.T) {
	const geoIPDB = "../../../docker/core/GeoLite2-Country.mmdb"
	if _, err := os.Stat(geoIPDB); err != nil {
		t.Skip("GeoLite2-Country.mmdb is not available; skipping country firewall test")
	}
	t.Setenv("SECURITY_GEOIP2", geoIPDB)

	setup, teardownTestCase := setupFirewall(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := firewallrpc.NewFirewallHandlersClient(setup)
		return a.IPAccess(ctx, req.(*firewallmessage.IPAccess_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &firewallmessage.IPAccess_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"client_ip": "value is required",
				},
			},
		},
		{ // request with broken parameters
			Name: "test0_02",
			Request: &firewallmessage.IPAccess_Request{
				ClientIp: "123",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"client_ip": "value must be a valid IP address",
				},
			},
		},
		{ // request with local ip: the country is unknown, so access fails closed
			Name: "test0_03",
			Request: &firewallmessage.IPAccess_Request{
				ClientIp: "127.0.0.1",
			},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Access is denied for this country",
			},
		},
		{ // request with an IP that is not present in the GeoIP database
			Name: "test0_03_1",
			Request: &firewallmessage.IPAccess_Request{
				ClientIp: "192.0.2.1",
			},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Access is denied for this country",
			},
		},
		{ // request with parameters
			Name: "test0_04",
			Request: &firewallmessage.IPAccess_Request{
				ClientIp: "64.233.164.102",
			},
			Response: test.BodyTable{
				"country_name": "United States",
				"country_code": "US",
			},
		},
		{ // request with blocked ip
			Name: "test0_05",
			Request: &firewallmessage.IPAccess_Request{
				ClientIp: "37.214.65.1",
			},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Access is denied for this country",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

// Test_IPAccessGeoIPUnavailable verifies that a missing or unreadable GeoIP
// database fails closed instead of allowing the request.
func Test_IPAccessGeoIPUnavailable(t *testing.T) {
	t.Setenv("SECURITY_GEOIP2", "../../../docker/core/missing.mmdb")

	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := firewallrpc.NewFirewallHandlersClient(setup)
		return a.IPAccess(ctx, req.(*firewallmessage.IPAccess_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name: "geoip_database_missing",
			Request: &firewallmessage.IPAccess_Request{
				ClientIp: "64.233.164.102",
			},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Access is denied for this country",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_UpdateFirewallListData(t *testing.T) {
	setup, teardownTestCase := setupFirewall(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := firewallrpc.NewFirewallHandlersClient(setup)
		return a.UpdateFirewallListData(ctx, req.(*firewallmessage.UpdateFirewallListData_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &firewallmessage.UpdateFirewallListData_Request{},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
