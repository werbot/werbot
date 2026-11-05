package event_test

import (
	"context"
	"testing"

	eventenum "github.com/werbot/werbot/internal/core/event/proto/enum"
	eventmessage "github.com/werbot/werbot/internal/core/event/proto/message"
	eventpb "github.com/werbot/werbot/internal/core/event/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"github.com/werbot/werbot/pkg/uuid"
	"google.golang.org/grpc/codes"
)

const (
	adminProfileUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.9; rv:35.0) Gecko/20100101 Firefox/35."
	adminProfileIP        = "2001:0db8:85a3:0000:0000:8a2e:0370:7334"
	projectEventUserAgent = "Mozilla/5.0 (Linux; U; Android 4.0.4; en-us; KFJWI Build/IMM76D) AppleWebKit/537.36 (KHTML, like Gecko) Silk/3.68 like Chrome/39.0.2171.93 Safari/537.36"
	projectEventIP        = "192.168.0.1"
	schemeEventUserAgent  = "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; LCJB; rv:11.0) like Gecko"
	schemeEventIP         = "192.168.1.1"

	schemeEventCount = 11
)

// unknownID points at no row created by the test.
var unknownID = uuid.New()

// eventEnv is the profile, project and scheme graph owned by a test.
type eventEnv struct {
	admin, user factory.Profile

	project      factory.Project
	emptyProject factory.Project

	scheme      factory.Scheme
	emptyScheme factory.Scheme

	adminEvents       []factory.Event
	adminProfileEvent factory.Event
	adminSessionID    string

	projectEvents  []factory.Event
	projectEvent   factory.Event
	projectSession string

	schemeEvents  []factory.Event
	schemeEvent   factory.Event
	schemeSession string
}

func setupEvent(t *testing.T) (*test.GRPCHandler, *eventEnv, func(t *testing.T)) {
	t.Helper()

	setup, teardownTestCase := test.GRPCWithoutFixtures(t)
	builder := factory.New(t, setup.Postgres.Conn().Conn)

	env := &eventEnv{}
	env.admin = builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	env.user = builder.Profile(factory.ProfileParams{Alias: "user", Role: 1})

	env.project = builder.Project(factory.ProjectParams{OwnerID: env.admin.ID, Title: "event project"})
	env.emptyProject = builder.Project(factory.ProjectParams{OwnerID: env.admin.ID, Title: "empty project"})

	env.scheme = builder.Scheme(factory.SchemeParams{ProjectID: env.project.ID, Title: "event scheme"})
	env.emptyScheme = builder.Scheme(factory.SchemeParams{ProjectID: env.project.ID, Title: "empty scheme"})

	// Two admin profile events keep the limit case meaningful.
	env.adminSessionID = uuid.New()
	env.adminProfileEvent = builder.Event(factory.EventParams{
		RelatedID:    env.admin.ID,
		PrimeSection: 1,
		Section:      1,
		Type:         9,
		SessionID:    env.adminSessionID,
		UserAgent:    adminProfileUserAgent,
		IP:           adminProfileIP,
	})
	env.adminEvents = append(env.adminEvents,
		env.adminProfileEvent,
		builder.Event(factory.EventParams{
			RelatedID:    env.admin.ID,
			PrimeSection: 1,
			Section:      1,
			Type:         10,
		}),
	)

	// Two project events.
	env.projectSession = uuid.New()
	env.projectEvent = builder.Event(factory.EventParams{
		RelatedID:    env.project.ID,
		PrimeSection: 2,
		Section:      1,
		Type:         1,
		SessionID:    env.projectSession,
		UserAgent:    projectEventUserAgent,
		IP:           projectEventIP,
	})
	env.projectEvents = append(env.projectEvents,
		env.projectEvent,
		builder.Event(factory.EventParams{
			RelatedID:    env.project.ID,
			PrimeSection: 2,
			Section:      1,
			Type:         2,
		}),
	)

	// Eleven scheme events.
	env.schemeSession = uuid.New()
	env.schemeEvent = builder.Event(factory.EventParams{
		RelatedID:    env.scheme.ID,
		PrimeSection: 3,
		Section:      1,
		Type:         1,
		SessionID:    env.schemeSession,
		UserAgent:    schemeEventUserAgent,
		IP:           schemeEventIP,
	})
	env.schemeEvents = append(env.schemeEvents, env.schemeEvent)
	for i := 1; i < schemeEventCount; i++ {
		env.schemeEvents = append(env.schemeEvents, builder.Event(factory.EventParams{
			RelatedID:    env.scheme.ID,
			PrimeSection: 3,
			Section:      1,
			Type:         2,
		}))
	}

	return setup, env, teardownTestCase
}

func Test_Events(t *testing.T) {
	setup, env, teardownTestCase := setupEvent(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := eventpb.NewEventHandlersClient(setup)
		return a.Events(ctx, req.(*eventpb.Events_Request))
	}

	adminProfileTotal := float64(len(env.adminEvents))
	projectTotal := float64(len(env.projectEvents))
	schemeTotal := float64(len(env.schemeEvents))

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
					ProfileId: env.user.ID,
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
				OwnerId: env.admin.ID,
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
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_ProfileId{
					ProfileId: env.admin.ID,
				},
			},
			Response: test.BodyTable{
				"total":    adminProfileTotal,
				"events.0": "*",
				"events.1": "*",
				"events.2": nil,

				"events.0.created_at": "*",
				"events.1.created_at": "*",
			},
		},
		{ // list of all profile events with limit
			Name: "test0_05",
			Request: &eventpb.Events_Request{
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_ProfileId{
					ProfileId: env.admin.ID,
				},
				Limit: 1,
			},
			Response: test.BodyTable{
				"total":    adminProfileTotal,
				"events.0": "*",
				"events.1": nil,
			},
		},

		{ // "non-existent profile UUID"
			Name: "test0_06",
			Request: &eventpb.Events_Request{
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_ProfileId{
					ProfileId: unknownID,
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
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: env.emptyProject.ID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Project not found",
			},
		},

		{ // guest: project has no events for this owner
			Name: "test0_08",
			Request: &eventpb.Events_Request{
				OwnerId: env.user.ID,
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: env.project.ID,
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
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: env.project.ID,
				},
			},
			Response: test.BodyTable{
				"total":    projectTotal,
				"events.0": "*",
				"events.1": "*",
				"events.2": nil,
			},
		},
		{ // guest: list of all project events
			Name: "test0_10",
			Request: &eventpb.Events_Request{
				OwnerId: env.user.ID,
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: env.project.ID,
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
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: env.project.ID,
				},
				Limit: 1,
			},
			Response: test.BodyTable{
				"total":    projectTotal,
				"events.0": "*",
				"events.1": nil,
			},
		},
		{ // non-existent project UUID
			Name: "test0_12",
			Request: &eventpb.Events_Request{
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_ProjectId{
					ProjectId: unknownID,
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
					ProjectId: unknownID,
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
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: env.emptyScheme.ID,
				},
			},
			Error: test.ErrGRPC{
				Code:    codes.NotFound,
				Message: "Scheme not found",
			},
		},
		{ // guest: server has no events for this owner
			Name: "test0_15",
			Request: &eventpb.Events_Request{
				OwnerId: env.user.ID,
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: env.scheme.ID,
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
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: env.scheme.ID,
				},
			},
			Response: test.BodyTable{
				"total": schemeTotal,
			},
		},
		{ // guest: list of all server events
			Name: "test0_17",
			Request: &eventpb.Events_Request{
				OwnerId: env.user.ID,
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: env.scheme.ID,
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
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: env.scheme.ID,
				},
				Limit: 1,
			},
			Response: test.BodyTable{
				"total":    schemeTotal,
				"events.0": "*",
				"events.1": nil,
			},
		},
		{ // non-existent server UUID
			Name: "test0_19",
			Request: &eventpb.Events_Request{
				OwnerId: env.admin.ID,
				RelatedId: &eventpb.Events_Request_SchemeId{
					SchemeId: unknownID,
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
					SchemeId: unknownID,
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
	setup, env, teardownTestCase := setupEvent(t)
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
				OwnerId: unknownID,
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
				OwnerId: unknownID,
				EventId: &eventpb.Event_Request_ProfileId{
					ProfileId: unknownID,
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
				OwnerId: env.admin.ID,
				EventId: &eventpb.Event_Request_ProfileId{
					ProfileId: unknownID,
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
				OwnerId: env.admin.ID,
				EventId: &eventpb.Event_Request_ProfileId{
					ProfileId: env.adminProfileEvent.ID,
				},
			},
			Response: test.BodyTable{
				"profile_id":         env.admin.ID,
				"section":            float64(1),
				"type":               float64(9),
				"session.id":         env.adminSessionID,
				"session.user_agent": adminProfileUserAgent,
				"session.ip":         adminProfileIP,
				"meta_data":          "e30=",
				"created_at":         "*",
			},
		},

		// project event
		{
			Name: "test2_01",
			Request: &eventpb.Event_Request{
				OwnerId: env.admin.ID,
				EventId: &eventpb.Event_Request_ProjectId{
					ProjectId: unknownID,
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
				OwnerId: env.admin.ID,
				EventId: &eventpb.Event_Request_ProjectId{
					ProjectId: env.projectEvent.ID,
				},
			},
			Response: test.BodyTable{
				"project_id":         env.project.ID,
				"section":            float64(1),
				"type":               float64(1),
				"session.id":         env.projectSession,
				"session.user_agent": projectEventUserAgent,
				"session.ip":         projectEventIP,
				"meta_data":          "e30=",
				"created_at":         "*",
			},
		},

		// scheme event
		{
			Name: "test3_01",
			Request: &eventpb.Event_Request{
				OwnerId: env.admin.ID,
				EventId: &eventpb.Event_Request_SchemeId{
					SchemeId: unknownID,
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
				OwnerId: env.admin.ID,
				EventId: &eventpb.Event_Request_SchemeId{
					SchemeId: env.schemeEvent.ID,
				},
			},
			Response: test.BodyTable{
				"scheme_id":          env.scheme.ID,
				"section":            float64(1),
				"type":               float64(1),
				"session.id":         env.schemeSession,
				"session.user_agent": schemeEventUserAgent,
				"session.ip":         schemeEventIP,
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
	setup, env, teardownTestCase := setupEvent(t)
	defer teardownTestCase(t)

	handler := func(ctx context.Context, req test.ProtoMessage) (test.ProtoMessage, error) {
		a := eventpb.NewEventHandlersClient(setup)
		return a.AddEvent(ctx, req.(*eventpb.AddEvent_Request))
	}

	session := func() *eventmessage.Session {
		return &eventmessage.Session{
			Id:        uuid.New(),
			UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
			Ip:        "192.168.1.1",
		}
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
				OwnerId: env.admin.ID,
				Section: &eventpb.AddEvent_Request_Profile{
					Profile: &eventmessage.Profile{
						Id:      env.admin.ID,
						Section: eventmessage.Profile_profile,
					},
				},
				Session:  session(),
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
		},
		{
			Name: "test1_02",
			Request: &eventpb.AddEvent_Request{
				OwnerId: unknownID,
				Section: &eventpb.AddEvent_Request_Profile{
					Profile: &eventmessage.Profile{
						Id:      unknownID,
						Section: eventmessage.Profile_profile,
					},
				},
				Session:  session(),
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
				OwnerId: unknownID,
				Section: &eventpb.AddEvent_Request_Profile{
					Profile: &eventmessage.Profile{
						Id:      "test",
						Section: eventmessage.Profile_profile,
					},
				},
				Session:  session(),
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
				OwnerId: env.admin.ID,
				Section: &eventpb.AddEvent_Request_Project{
					Project: &eventmessage.Project{
						Id:      env.project.ID,
						Section: eventmessage.Project_setting,
					},
				},
				Session:  session(),
				Type:     eventenum.Type_onActive,
				MetaData: []byte("{\"test\":1}"),
			},
		},
		{
			Name: "test2_02",
			Request: &eventpb.AddEvent_Request{
				OwnerId: unknownID,
				Section: &eventpb.AddEvent_Request_Project{
					Project: &eventmessage.Project{
						Id:      unknownID,
						Section: eventmessage.Project_setting,
					},
				},
				Session:  session(),
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
				OwnerId: unknownID,
				Section: &eventpb.AddEvent_Request_Project{
					Project: &eventmessage.Project{
						Id:      "test",
						Section: eventmessage.Project_setting,
					},
				},
				Session:  session(),
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
				OwnerId: env.admin.ID,
				Section: &eventpb.AddEvent_Request_Scheme{
					Scheme: &eventmessage.Scheme{
						Id:      env.scheme.ID,
						Section: eventmessage.Scheme_setting,
					},
				},
				Session: session(),
				Type:    eventenum.Type_onActive,
			},
		},
		{
			Name: "test3_02",
			Request: &eventpb.AddEvent_Request{
				OwnerId: unknownID,
				Section: &eventpb.AddEvent_Request_Scheme{
					Scheme: &eventmessage.Scheme{
						Id:      unknownID,
						Section: eventmessage.Scheme_setting,
					},
				},
				Session:  session(),
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
				OwnerId: unknownID,
				Section: &eventpb.AddEvent_Request_Scheme{
					Scheme: &eventmessage.Scheme{
						Id:      "test",
						Section: eventmessage.Scheme_setting,
					},
				},
				Session:  session(),
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
