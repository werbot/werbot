package scheme_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Test_SystemSchemesByAlias(t *testing.T) {
	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	project := builder.Project(factory.ProjectParams{
		OwnerID: admin.ID,
		Alias:   "C8cXx0",
		Title:   "project",
	})
	member := builder.ProjectMember(factory.ProjectMemberParams{ProjectID: project.ID, ProfileID: admin.ID})

	access := func(alias string) map[string]any {
		return map[string]any{
			"alias":   alias,
			"address": "127.0.0.1",
			"port":    9300,
			"access": map[string]any{
				"login":    "base4",
				"password": "password4",
			},
		}
	}

	target := builder.Scheme(factory.SchemeParams{
		ProjectID:  project.ID,
		SchemeType: 205,
		Title:      "Elastic server #1",
		Access:     access("7TrR6t"),
	})
	targetMember := builder.SchemeMember(factory.SchemeMemberParams{SchemeID: target.ID, ProjectMemberID: member.ID})

	// The remaining schemes keep the profile total at 14 but do not match the
	// fully qualified alias.
	for i := 1; i < 14; i++ {
		s := builder.Scheme(factory.SchemeParams{
			ProjectID:  project.ID,
			SchemeType: 205,
			Title:      "scheme",
			Access:     access(fmt.Sprintf("alias%02d", i)),
		})
		builder.SchemeMember(factory.SchemeMemberParams{SchemeID: s.ID, ProjectMemberID: member.ID})
	}

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := schemepb.NewSchemeHandlersClient(setup)
		return a.SystemSchemesByAlias(ctx, req.(*schemepb.SystemSchemesByAlias_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &schemepb.SystemSchemesByAlias_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"alias": "value is required",
				},
			},
		},
		{ // request with parameters small login
			Name: "test0_02",
			Request: &schemepb.SystemSchemesByAlias_Request{
				Alias: "a",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"alias": "value length must be at least 3 characters",
				},
			},
		},
		{ // request with parameters broken symbol login
			Name: "test0_03",
			Request: &schemepb.SystemSchemesByAlias_Request{
				Alias: "adm!n",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"alias": "value does not match regex pattern `^[a-zA-Z0-9_]+$`",
				},
			},
		},
		{ // only broken user alias
			Name: "test0_04",
			Request: &schemepb.SystemSchemesByAlias_Request{
				Alias: "test",
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{ // only user alias
			Name: "test0_05",
			Request: &schemepb.SystemSchemesByAlias_Request{
				Alias: "admin",
			},
			Response: test.BodyTable{
				"total":   float64(14),
				"schemes": "*",
			},
		},
		{ // user and broken project alias
			Name: "test0_06",
			Request: &schemepb.SystemSchemesByAlias_Request{
				Alias: "admin_test",
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{ // user and project alias
			Name: "test0_07",
			Request: &schemepb.SystemSchemesByAlias_Request{
				Alias: "admin_C8cXx0",
			},
			Response: test.BodyTable{
				"total":   float64(14),
				"schemes": "*",
			},
		},
		{ // user, project and broken scheme alias
			Name: "test0_08",
			Request: &schemepb.SystemSchemesByAlias_Request{
				Alias: "admin_C8cXx0_test",
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{ // user, project and scheme alias
			Name: "test0_09",
			Request: &schemepb.SystemSchemesByAlias_Request{
				Alias: "admin_C8cXx0_7TrR6t",
			},
			Response: test.BodyTable{
				"total":                      float64(1),
				"schemes.0.project_id":       project.ID,
				"schemes.0.scheme_id":        target.ID,
				"schemes.0.scheme_member_id": targetMember.ID,
				"schemes.0.scheme_type":      float64(205),
				"schemes.0.auth_method":      float64(1),
				"schemes.0.alias":            "admin_C8cXx0_7TrR6t",
				"schemes.0.title":            "Elastic server #1",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_SystemSchemeAccess(t *testing.T) {
	const geoIPDB = "../../../docker/core/GeoLite2-Country.mmdb"
	if _, err := os.Stat(geoIPDB); err != nil {
		t.Skip("GeoLite2-Country.mmdb is not available; skipping country access test")
	}
	t.Setenv("SECURITY_GEOIP2", geoIPDB)

	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)

	listID := builder.FirewallList(factory.FirewallListParams{Name: "global"})
	builder.FirewallCountry(listID, "RU")
	builder.FirewallCountry(listID, "BY")
	builder.FirewallNetwork(listID, "178.239.2.11/32")

	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID, Title: "project1"})
	ssh := builder.Scheme(factory.SchemeParams{
		ProjectID: project.ID,
		Title:     "Git server",
		Access: map[string]any{
			"alias":   "onxzU5",
			"address": "secret.net.google.com",
			"port":    2206,
			"password": map[string]any{
				"login":    "ubuntu6",
				"password": "ubuntu6",
			},
		},
	})

	week := weekGrid(24, 1)
	week["mon"][0] = 0
	builder.SchemeActivity(ssh.ID, week)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := schemepb.NewSchemeHandlersClient(setup)
		return a.SystemSchemeAccess(ctx, req.(*schemepb.SystemSchemeAccess_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &schemepb.SystemSchemeAccess_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"scheme_id": "value is required",
					"client_ip": "value is required",
				},
			},
		},
		{ // request with broken parameters
			Name: "test0_02",
			Request: &schemepb.SystemSchemeAccess_Request{
				SchemeId: "123",
				ClientIp: "321",
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"scheme_id": "value must be a valid UUID",
					"client_ip": "value must be a valid IP address",
				},
			},
		},
		{ // request with global blocked ip
			Name: "test0_03",
			Request: &schemepb.SystemSchemeAccess_Request{
				SchemeId: ssh.ID,
				ClientIp: "178.239.2.11",
			},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Access is denied for this ip",
			},
		},
		{ // request with global blocked country
			Name: "test0_04",
			Request: &schemepb.SystemSchemeAccess_Request{
				SchemeId: ssh.ID,
				ClientIp: "86.57.251.89",
			},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Access is denied for this country",
			},
		},
		{ // request with blocked time
			Name: "test0_05",
			Request: &schemepb.SystemSchemeAccess_Request{
				SchemeId: ssh.ID,
				ClientIp: "64.233.162.100",
				Timestamp: &timestamppb.Timestamp{ // 2024-09-16 00:18:09.455761+00
					Seconds: 1726445889,
					Nanos:   455761000,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.PermissionDenied,
				Message: "Access is denied for this time",
			},
		},
		{ // request
			Name: "test0_06",
			Request: &schemepb.SystemSchemeAccess_Request{
				SchemeId: ssh.ID,
				ClientIp: "64.233.162.100",
				Timestamp: &timestamppb.Timestamp{ // 2024-09-16 01:18:09.455761+00
					Seconds: 1726449489,
					Nanos:   455761000,
				},
			},
			Response: test.BodyTable{
				"project_id":                          project.ID,
				"scheme_type":                         float64(103),
				"access.server_ssh.alias":             "onxzU5",
				"access.server_ssh.password.password": "***",
			},
		},
		{ // last hour of a 24 element grid is allowed
			Name: "test0_07",
			Request: &schemepb.SystemSchemeAccess_Request{
				SchemeId: ssh.ID,
				ClientIp: "64.233.162.100",
				Timestamp: &timestamppb.Timestamp{ // 2024-09-16 23:18:09.455761+00
					Seconds: 1726528689,
					Nanos:   455761000,
				},
			},
			Response: test.BodyTable{
				"project_id":                          project.ID,
				"scheme_type":                         float64(103),
				"access.server_ssh.alias":             "onxzU5",
				"access.server_ssh.password.password": "***",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

// Test_SystemSchemeAccessGeoIPUnavailable verifies that an unavailable GeoIP
// database denies scheme access instead of failing open.
func Test_SystemSchemeAccessGeoIPUnavailable(t *testing.T) {
	t.Setenv("SECURITY_GEOIP2", "../../../docker/core/missing.mmdb")

	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID})
	ssh := builder.Scheme(factory.SchemeParams{ProjectID: project.ID})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := schemepb.NewSchemeHandlersClient(setup)
		return a.SystemSchemeAccess(ctx, req.(*schemepb.SystemSchemeAccess_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name: "geoip_database_missing",
			Request: &schemepb.SystemSchemeAccess_Request{
				SchemeId: ssh.ID,
				ClientIp: "64.233.162.100",
				Timestamp: &timestamppb.Timestamp{
					Seconds: 1726449489,
					Nanos:   455761000,
				},
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

func Test_SystemHostKey(t *testing.T) {
	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID})

	withKey := builder.Scheme(factory.SchemeParams{ProjectID: project.ID, Title: "with key"})
	withoutKey := builder.Scheme(factory.SchemeParams{ProjectID: project.ID, Title: "without key"})
	builder.SchemeHostKey(withKey.ID, []byte("test\n"))

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := schemepb.NewSchemeHandlersClient(setup)
		return a.SystemHostKey(ctx, req.(*schemepb.SystemHostKey_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &schemepb.SystemHostKey_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"scheme_id": "value is required",
				},
			},
		},
		{ // request
			Name: "test0_02",
			Request: &schemepb.SystemHostKey_Request{
				SchemeId: withKey.ID,
			},
			Response: test.BodyTable{
				"hostkey": "dGVzdAo=",
			},
		},
		{ // request
			Name: "test0_03",
			Request: &schemepb.SystemHostKey_Request{
				SchemeId: withoutKey.ID,
			},
			Response: test.BodyTable{
				"hostkey": nil,
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_SystemUpdateHostKey(t *testing.T) {
	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	defer teardownTestCase(t)

	builder := factory.New(t, setup.Postgres.Conn().Conn)
	admin := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	project := builder.Project(factory.ProjectParams{OwnerID: admin.ID})
	ssh := builder.Scheme(factory.SchemeParams{ProjectID: project.ID, Title: "Git server"})

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := schemepb.NewSchemeHandlersClient(setup)
		return a.SystemUpdateHostKey(ctx, req.(*schemepb.SystemUpdateHostKey_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &schemepb.SystemUpdateHostKey_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"scheme_id": "value is required",
					"hostkey":   "value is required",
				},
			},
		},
		{ // request
			Name: "test0_02",
			Request: &schemepb.SystemUpdateHostKey_Request{
				SchemeId: ssh.ID,
				Hostkey:  []byte("message"),
			},
			Response: test.BodyTable{},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
