package firewall_test

import (
	"context"
	"testing"

	firewallpb "github.com/werbot/werbot/internal/core/firewall/proto/firewall"
	"github.com/werbot/werbot/internal/utils/test"
	"google.golang.org/grpc/codes"
)

func Test_IPAccess(t *testing.T) {
	// t.Setenv("ENV_MODE", "test")
	t.Setenv("SECURITY_GEOIP2", "../../../docker/core/GeoLite2-Country.mmdb")

	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := firewallpb.NewFirewallHandlersClient(setup)
		return a.IPAccess(ctx, req.(*firewallpb.IPAccess_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &firewallpb.IPAccess_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"client_ip": "value is required",
				},
			},
		},
		{ // request with broken parameters
			Name: "test0_02",
			Request: &firewallpb.IPAccess_Request{
				ClientIp: "123",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"client_ip": "value must be a valid IP address",
				},
			},
		},
		{ // request with local ip
			Name: "test0_03",
			Request: &firewallpb.IPAccess_Request{
				ClientIp: "127.0.0.1",
			},
			Response: test.BodyTable{},
		},
		{ // request with parameters
			Name: "test0_04",
			Request: &firewallpb.IPAccess_Request{
				ClientIp: "64.233.164.102",
			},
			Response: test.BodyTable{
				"country_name": "United States",
				"country_code": "US",
			},
		},
		{ // request with blocked ip
			Name: "test0_05",
			Request: &firewallpb.IPAccess_Request{
				ClientIp: "37.214.65.1",
			},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Access is denied for this country",
			},
		},
	}

	test.RunCaseGRPCTests(t, handler, testTable)
}

func Test_UpdateFirewallListData(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := firewallpb.NewFirewallHandlersClient(setup)
		return a.UpdateFirewallListData(ctx, req.(*firewallpb.UpdateFirewallListData_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &firewallpb.UpdateFirewallListData_Request{},
		},
	}

	test.RunCaseGRPCTests(t, handler, testTable)
}
