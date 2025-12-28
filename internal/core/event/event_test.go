package event_test

import (
	"context"
	"testing"

	eventenum "github.com/werbot/werbot/internal/core/event/proto/enum"
	eventmessage "github.com/werbot/werbot/internal/core/event/proto/message"
	eventpb "github.com/werbot/werbot/internal/core/event/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/uuid"
	"google.golang.org/grpc/codes"
)

func Test_Events(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := eventpb.NewEventHandlersClient(setup)
		return a.Events(ctx, req.(*eventpb.Events_Request))
	}

	testTable := []test.GRPCTable{
		{ // request without parameters
			Name:    "test0_01",
			Request: &eventpb.Events_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":   "value is required",
					"related_id": "exactly one field is required in oneof",
				},
			},
		},
		{
			Name: "test0_02",
			Request: &eventpb.Events_Request{
				RelatedId: &eventpb.Events_Request_ProfileId{
					ProfileId: test.ConstUserID,
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value is required",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &eventpb.Events_Request{
				OwnerId: test.ConstAdminID,
				RelatedId: &eventpb.Events_Request_ProfileId{
					ProfileId: "test",
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile_id": "value must be a valid UUID",
				},
			},
		},

		{ // list of all admin profile events
			Name: "test0_04",
			Request: &eventpb.Events_Request{
				OwnerId: test.ConstAdminID,
				RelatedId: &eventpb.Events_Request_ProfileId{
					ProfileId: test.ConstAdminID,
				},
			},
			Response: test.BodyTable{
				"total":    float64(2),
				"events.0": "*",
				"events.1": "*",
				"events.2": nil,
			},
		},
		{ // list of all profile events with limit
			Name: "test0_05",
			Request: &eventpb.Events_Request{
				OwnerId: test.ConstAdminID,
				RelatedId: &eventpb.Events_Request_ProfileId{
					ProfileId: test.ConstAdminID,
				},
				Limit: 1,
			},
			Response: test.BodyTable{
				"total":    float64(2),
				"events.0": "*",
				"events.1": nil,
			},
		},

		{ // "non-existent profile UUID"
			Name: "test0_06",
			Request: &eventpb.Events_Request{
				OwnerId: test.ConstAdminID,
				RelatedId: &eventpb.Events_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Profile not found",
			},
		},

		// project events
		{ // "owner: project has no events"
			Name: "test0_07",
			Request: &eventpb.Events_Request{
				OwnerId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: "ca7e65a4-76ea-4802-9f4f-3518a3416985",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},

		{ // guest: project has no events
			Name: "test0_08",
			Request: &eventpb.Events_Request{
				OwnerId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: "ca7e65a4-76ea-4802-9f4f-3518a3416985",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},
		{ // owner: list of all project events
			Name: "test0_09",
			Request: &eventpb.Events_Request{
				OwnerId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: "26060c68-5a06-4a57-b87a-be0f1e787157",
				},
			},
			Response: test.BodyTable{
				"total":    float64(2),
				"events.0": "*",
				"events.1": "*",
				"events.2": nil,
			},
		},
		{ // guest: list of all project events
			Name: "test0_10",
			Request: &eventpb.Events_Request{
				OwnerId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: "26060c68-5a06-4a57-b87a-be0f1e787157",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},
		{ // owner: list of all project events with limit
			Name: "test0_11",
			Request: &eventpb.Events_Request{
				OwnerId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: "26060c68-5a06-4a57-b87a-be0f1e787157",
				},
				Limit: 1,
			},
			Response: test.BodyTable{
				"total":    float64(2),
				"events.0": "*",
				"events.1": nil,
			},
		},
		{ // non-existent project UUID
			Name: "test0_12",
			Request: &eventpb.Events_Request{
				OwnerId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},
		{ // project events no custom UUID passed
			Name: "test0_13",
			Request: &eventpb.Events_Request{
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value is required",
				},
			},
		},

		// server events
		{ // owner: server has no events
			Name: "test0_14",
			Request: &eventpb.Events_Request{
				OwnerId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: "ddd084a5-7d91-4796-a133-feab4e653721",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{ // guest: server has no events
			Name: "test0_15",
			Request: &eventpb.Events_Request{
				OwnerId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: "ca7e65a4-76ea-4802-9f4f-3518a3416985",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{ // owner: list of all server events
			Name: "test0_16",
			Request: &eventpb.Events_Request{
				OwnerId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: "0c3a8869-6fc0-4666-bf60-15475473392a",
				},
			},
			Response: test.BodyTable{
				"total": float64(11),
			},
		},
		{ // guest: list of all server events
			Name: "test0_17",
			Request: &eventpb.Events_Request{
				OwnerId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: "0c3a8869-6fc0-4666-bf60-15475473392a",
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{ // owner: list of all server events with limit
			Name: "test0_18",
			Request: &eventpb.Events_Request{
				OwnerId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: "0c3a8869-6fc0-4666-bf60-15475473392a",
				},
				Limit: 1,
			},
			Response: test.BodyTable{
				"total":    float64(11),
				"events.0": "*",
				"events.1": nil,
			},
		},
		{ // non-existent server UUID
			Name: "test0_19",
			Request: &eventpb.Events_Request{
				OwnerId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{ // server event no custom UUID passed
			Name: "test0_20",
			Request: &eventpb.Events_Request{
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value is required",
				},
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_Event(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := eventpb.NewEventHandlersClient(setup)
		return a.Event(ctx, req.(*eventpb.Event_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "test0_01",
			Request: &eventpb.Event_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value is required",
					"event_id": "exactly one field is required in oneof",
				},
			},
		},
		{
			Name: "test0_02",
			Request: &eventpb.Event_Request{
				OwnerId: test.ConstFakeID,
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"event_id": "exactly one field is required in oneof",
				},
			},
		},
		{
			Name: "test0_03",
			Request: &eventpb.Event_Request{
				OwnerId: test.ConstFakeID,
				EventId: &eventpb.Event_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},

		// profile event
		{
			Name: "test1_01",
			Request: &eventpb.Event_Request{
				OwnerId: test.ConstAdminID,
				EventId: &eventpb.Event_Request_ProfileId{
					ProfileId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test1_02",
			Request: &eventpb.Event_Request{
				OwnerId: test.ConstAdminID,
				EventId: &eventpb.Event_Request_ProfileId{
					ProfileId: test.ConstAdminProfileEventID,
				},
			},
			Response: test.BodyTable{
				"profile_id":         test.ConstAdminID,
				"section":            float64(1),
				"type":               float64(9),
				"session.id":         "98e3ddfc-dab0-4d4e-b48e-ab1717acae8b",
				"session.user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.9; rv:35.0) Gecko/20100101 Firefox/35.",
				"session.ip":         "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
				"meta_data":          "e30=",
				"created_at":         "*",
			},
		},

		// project event
		{
			Name: "test2_01",
			Request: &eventpb.Event_Request{
				OwnerId: test.ConstAdminID,
				EventId: &eventpb.Event_Request_ProjectId{
					ProjectId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test2_02",
			Request: &eventpb.Event_Request{
				OwnerId: test.ConstAdminID,
				EventId: &eventpb.Event_Request_ProjectId{
					ProjectId: test.ConstAdminProjectEventID,
				},
			},
			Response: test.BodyTable{
				"project_id":         test.ConstAdminEventProjectID,
				"section":            float64(1),
				"type":               float64(1),
				"session.id":         "98e3ddfc-dab0-4d4e-b48e-ab1717acae8b",
				"session.user_agent": "Mozilla/5.0 (Linux; U; Android 4.0.4; en-us; KFJWI Build/IMM76D) AppleWebKit/537.36 (KHTML, like Gecko) Silk/3.68 like Chrome/39.0.2171.93 Safari/537.36",
				"session.ip":         "192.168.0.1",
				"meta_data":          "e30=",
				"created_at":         "*",
			},
		},

		// scheme event
		{
			Name: "test3_01",
			Request: &eventpb.Event_Request{
				OwnerId: test.ConstAdminID,
				EventId: &eventpb.Event_Request_SchemeId{
					SchemeId: test.ConstFakeID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test3_02",
			Request: &eventpb.Event_Request{
				OwnerId: test.ConstAdminID,
				EventId: &eventpb.Event_Request_SchemeId{
					SchemeId: test.ConstAdminSchemeEventID,
				},
			},
			Response: test.BodyTable{
				"scheme_id":          test.ConstAdminEventSchemeID,
				"section":            float64(1),
				"type":               float64(1),
				"session.id":         "98e3ddfc-dab0-4d4e-b48e-ab1717acae8b",
				"session.user_agent": "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; LCJB; rv:11.0) like Gecko",
				"session.ip":         "192.168.1.1",
				"meta_data":          "e30=",
				"created_at":         "*",
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}

func Test_AddEvent(t *testing.T) {
	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := eventpb.NewEventHandlersClient(setup)
		return a.AddEvent(ctx, req.(*eventpb.AddEvent_Request))
	}

	testTable := []test.GRPCTable{
		{
			Name:    "test0_01",
			Request: &eventpb.AddEvent_Request{},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id": "value is required",
					"section":  "exactly one field is required in oneof",
					"type":     "value is required",
				},
			},
		},
		{
			Name: "test0_02",
			Request: &eventpb.AddEvent_Request{
				OwnerId: "test",
				Section: &eventpb.AddEvent_Request_Profile{
					Profile: &eventmessage.Profile{
						Id:      "test",
						Section: eventmessage.Profile_profile,
					},
				},
				Session: &eventmessage.Session{
					Id:        "test",
					UserAgent: "test",
					Ip:        "test",
				},
				Type:     eventenum.Type_onActive,
				MetaData: []byte("test"),
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"owner_id":   "value must be a valid UUID",
					"profile.id": "value must be a valid UUID",
					"session.id": "value must be a valid UUID",
					"session.ip": "value must be a valid IP address",
				},
			},
		},

		// profile section
		{
			Name: "test1_01",
			Request: &eventpb.AddEvent_Request{
				OwnerId: test.ConstAdminID,
				Section: &eventpb.AddEvent_Request_Profile{
					Profile: &eventmessage.Profile{
						Id:      test.ConstAdminID,
						Section: eventmessage.Profile_profile,
					},
				},
				Session: &eventmessage.Session{
					Id:        uuid.New(),
					UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
					Ip:        "192.168.1.1",
				},
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
		},
		{
			Name: "test1_02",
			Request: &eventpb.AddEvent_Request{
				OwnerId: test.ConstFakeID,
				Section: &eventpb.AddEvent_Request_Profile{
					Profile: &eventmessage.Profile{
						Id:      test.ConstFakeID,
						Section: eventmessage.Profile_profile,
					},
				},
				Session: &eventmessage.Session{
					Id:        uuid.New(),
					UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
					Ip:        "192.168.1.1",
				},
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test1_03",
			Request: &eventpb.AddEvent_Request{
				OwnerId: test.ConstFakeID,
				Section: &eventpb.AddEvent_Request_Profile{
					Profile: &eventmessage.Profile{
						Id:      "test",
						Section: eventmessage.Profile_profile,
					},
				},
				Session: &eventmessage.Session{
					Id:        uuid.New(),
					UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
					Ip:        "192.168.1.1",
				},
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"profile.id": "value must be a valid UUID",
				},
			},
		},

		// project section
		{
			Name: "test2_01",
			Request: &eventpb.AddEvent_Request{
				OwnerId: test.ConstAdminID,
				Section: &eventpb.AddEvent_Request_Project{
					Project: &eventmessage.Project{
						Id:      test.ConstAdminEventProjectID,
						Section: eventmessage.Project_setting,
					},
				},
				Session: &eventmessage.Session{
					Id:        uuid.New(),
					UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
					Ip:        "192.168.1.1",
				},
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
		},
		{
			Name: "test2_02",
			Request: &eventpb.AddEvent_Request{
				OwnerId: test.ConstFakeID,
				Section: &eventpb.AddEvent_Request_Project{
					Project: &eventmessage.Project{
						Id:      test.ConstFakeID,
						Section: eventmessage.Project_setting,
					},
				},
				Session: &eventmessage.Session{
					Id:        uuid.New(),
					UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
					Ip:        "192.168.1.1",
				},
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test2_03",
			Request: &eventpb.AddEvent_Request{
				OwnerId: test.ConstFakeID,
				Section: &eventpb.AddEvent_Request_Project{
					Project: &eventmessage.Project{
						Id:      "test",
						Section: eventmessage.Project_setting,
					},
				},
				Session: &eventmessage.Session{
					Id:        uuid.New(),
					UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
					Ip:        "192.168.1.1",
				},
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"project.id": "value must be a valid UUID",
				},
			},
		},

		// scheme section
		{
			Name: "test3_01",
			Request: &eventpb.AddEvent_Request{
				OwnerId: test.ConstAdminID,
				Section: &eventpb.AddEvent_Request_Scheme{
					Scheme: &eventmessage.Scheme{
						Id:      test.ConstAdminEventSchemeID,
						Section: eventmessage.Scheme_setting,
					},
				},
				Session: &eventmessage.Session{
					Id:        uuid.New(),
					UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
					Ip:        "192.168.1.1",
				},
				Type: eventenum.Type_onActive,
				// MetaData:  []byte("{\"test\":1}"),
			},
		},
		{
			Name: "test3_02",
			Request: &eventpb.AddEvent_Request{
				OwnerId: test.ConstFakeID,
				Section: &eventpb.AddEvent_Request_Scheme{
					Scheme: &eventmessage.Scheme{
						Id:      test.ConstFakeID,
						Section: eventmessage.Scheme_setting,
					},
				},
				Session: &eventmessage.Session{
					Id:        uuid.New(),
					UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
					Ip:        "192.168.1.1",
				},
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Not found",
			},
		},
		{
			Name: "test3_03",
			Request: &eventpb.AddEvent_Request{
				OwnerId: test.ConstFakeID,
				Section: &eventpb.AddEvent_Request_Scheme{
					Scheme: &eventmessage.Scheme{
						Id:      "test",
						Section: eventmessage.Scheme_setting,
					},
				},
				Session: &eventmessage.Session{
					Id:        uuid.New(),
					UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
					Ip:        "192.168.1.1",
				},
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
			Error: test.ErrGRPC{
				Code: codes.InvalidArgument,
				Message: map[string]any{
					"scheme.id": "value must be a valid UUID",
				},
			},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			test.RunCaseGRPCTests(t, handler, tt)
		})
	}
}
