package scheme_test

import (
	"context"
	"testing"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/utils/test"
	"google.golang.org/grpc/codes"
)

func Test_ProfileSchemes(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := schemepb.NewSchemeHandlersClient(setup)
		return a.ProfileSchemes(ctx, req.(*schemepb.ProfileSchemes_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &schemepb.ProfileSchemes_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value is empty, which is not a valid UUID",
				},
			},
		},
		{ // user schemes
			Name: "test0_02",
			Request: &schemepb.ProfileSchemes_Request{
				ProfileId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
			},
			Response: test.BodyTable{
				"total.100":            float64(5),
				"total.200":            float64(10),
				"total.300":            float64(2),
				"total.400":            float64(3),
				"total.500":            float64(8),
				"total.600":            float64(2),
				"schemes.0.project_id": "2bef1080-cd6e-49e5-8042-1224cf6a3da9",
			},
		},
		{ // user database schemes
			// Debug: true,
			Name: "test0_03",
			Request: &schemepb.ProfileSchemes_Request{
				ProfileId:  "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				SchemeType: schemeaccesspb.SchemeType_database,
			},
			Response: test.BodyTable{
				"total.100":            float64(5),
				"total.200":            float64(10),
				"total.300":            float64(2),
				"total.400":            float64(3),
				"total.500":            float64(8),
				"total.600":            float64(2),
				"schemes.0.project_id": "2bef1080-cd6e-49e5-8042-1224cf6a3da9",
			},
		},

		{ // admin schemes
			// Debug: true,
			Name: "test0_04",
			Request: &schemepb.ProfileSchemes_Request{
				ProfileId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			Response: test.BodyTable{
				"total.100": float64(5),
				"total.200": float64(7),
				"total.300": float64(2),
				"total.400": float64(2),
				"total.500": float64(0),
				"total.600": float64(0),
			},
		},

		{ // user1 schemes
			Name: "test0_05",
			Request: &schemepb.ProfileSchemes_Request{
				ProfileId: "b3dc36e2-7f84-414b-b147-7ac850369518",
			},
			Response: test.BodyTable{
				"total.100": float64(1),
				"total.200": float64(0),
				"total.300": float64(0),
				"total.400": float64(0),
				"total.500": float64(0),
				"total.600": float64(0),
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
