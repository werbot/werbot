package event_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	eventpb "github.com/werbot/werbot/internal/grpc/event/proto"
	"github.com/werbot/werbot/internal/utils/test"
)

type testSetup struct {
	ctx  context.Context
	grpc *grpc.ClientConn
}

func setupTest(t *testing.T) (testSetup, func(t *testing.T)) {
	ctx := context.Background()

	postgres, err := test.Postgres(t, "../../../migration", "../../../fixtures/migration")
	if err != nil {
		t.Error(err)
	}

	grpc, err := test.GRPC(ctx, t, postgres.Conn, nil)
	if err != nil {
		t.Error(err)
	}

	return testSetup{
			ctx:  ctx,
			grpc: grpc.ClientConn,
		}, func(t *testing.T) {
			postgres.Close()
			grpc.Close()
		}
}

func Test_Events(t *testing.T) {
	//t.Parallel()
	setup, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testCases := []struct {
		name    string
		req     *eventpb.Events_Request
		resp    *eventpb.Events_Response
		respErr string
	}{
		{
			name:    "request without parameters",
			req:     &eventpb.Events_Request{},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},

		// profile events
		{
			name: "profile has no events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProfileId{
					ProfileId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				},
			},
			resp: &eventpb.Events_Response{
				Total: 0,
			},
		},
		{
			name: "list of all profile events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProfileId{
					ProfileId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				},
			},
			resp: &eventpb.Events_Response{
				Total: 2,
			},
		},
		{
			name: "list of all profile events with limit",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProfileId{
					ProfileId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				},
				Limit: 1,
			},
			resp: &eventpb.Events_Response{
				Total: 2,
			},
		},
		{
			name: "non-existent profile UUID",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProfileId{
					ProfileId: "00000000-0000-0000-0000-000000000000",
				},
			},
			resp: &eventpb.Events_Response{},
		},

		// project events
		{
			name: "owner: project has no events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProjectId{
					ProjectId: "ca7e65a4-76ea-4802-9f4f-3518a3416985",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &eventpb.Events_Response{
				Total: 0,
			},
		},
		{
			name: "guest: project has no events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProjectId{
					ProjectId: "ca7e65a4-76ea-4802-9f4f-3518a3416985",
				},
				UserId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
			},
			resp: &eventpb.Events_Response{
				Total: 0,
			},
		},
		{
			name: "owner: list of all project events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProjectId{
					ProjectId: "26060c68-5a06-4a57-b87a-be0f1e787157",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &eventpb.Events_Response{
				Total: 2,
			},
		},
		{
			name: "guest: list of all project events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProjectId{
					ProjectId: "26060c68-5a06-4a57-b87a-be0f1e787157",
				},
				UserId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
			},
			resp: &eventpb.Events_Response{
				Total: 0,
			},
		},
		{
			name: "owner: list of all project events with limit",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProjectId{
					ProjectId: "26060c68-5a06-4a57-b87a-be0f1e787157",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				Limit:  1,
			},
			resp: &eventpb.Events_Response{
				Total: 2,
			},
		},
		{
			name: "non-existent project UUID",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProjectId{
					ProjectId: "00000000-0000-0000-0000-000000000000",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &eventpb.Events_Response{},
		},
		{
			name: "project events no custom UUID passed",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ProjectId{
					ProjectId: "00000000-0000-0000-0000-000000000000",
				},
			},
			resp:    &eventpb.Events_Response{},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},

		// server events
		{
			name: "owner: server has no events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ServerId{
					ServerId: "ddd084a5-7d91-4796-a133-feab4e653721",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &eventpb.Events_Response{
				Total: 0,
			},
		},
		{
			name: "guest: server has no events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ServerId{
					ServerId: "ca7e65a4-76ea-4802-9f4f-3518a3416985",
				},
				UserId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
			},
			resp: &eventpb.Events_Response{
				Total: 0,
			},
		},
		{
			name: "owner: list of all server events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ServerId{
					ServerId: "0c3a8869-6fc0-4666-bf60-15475473392a",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &eventpb.Events_Response{
				Total: 11,
			},
		},
		{
			name: "guest: list of all server events",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ServerId{
					ServerId: "0c3a8869-6fc0-4666-bf60-15475473392a",
				},
				UserId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
			},
			resp: &eventpb.Events_Response{
				Total: 0,
			},
		},
		{
			name: "owner: list of all server events with limit",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ServerId{
					ServerId: "0c3a8869-6fc0-4666-bf60-15475473392a",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				Limit:  1,
			},
			resp: &eventpb.Events_Response{
				Total: 11,
			},
		},
		{
			name: "non-existent server UUID",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ServerId{
					ServerId: "00000000-0000-0000-0000-000000000000",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &eventpb.Events_Response{},
		},
		{
			name: "server event no custom UUID passed",
			req: &eventpb.Events_Request{
				Id: &eventpb.Events_Request_ServerId{
					ServerId: "00000000-0000-0000-0000-000000000000",
				},
			},
			resp:    &eventpb.Events_Response{},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			a := eventpb.NewEventHandlersClient(setup.grpc)
			response, err := a.Events(setup.ctx, tt.req)
			if err != nil {
				assert.EqualError(t, err, tt.respErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.resp.GetTotal(), response.GetTotal())
		})
	}
}

func Test_Event(t *testing.T) {
	//t.Parallel()
	setup, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testCases := []struct {
		name    string
		req     *eventpb.Event_Request
		resp    *eventpb.Event_Response
		respErr string
	}{
		{
			name:    "request without parameters",
			req:     &eventpb.Event_Request{},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},

		// profile event
		{
			name: "owner: show profile event by UUID",
			req: &eventpb.Event_Request{
				Id: &eventpb.Event_Request_ProfileId{
					ProfileId: "59fab0fa-8f0a-4065-8863-0dae40166015",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &eventpb.Event_Response{
				Ip:       "2001:db8:85a3::8a2e:370:7334",
				Event:    1,
				MetaData: []byte("{}"),
			},
		},
		{
			name: "owner: show profile event by fake UUID",
			req: &eventpb.Event_Request{
				Id: &eventpb.Event_Request_ProfileId{
					ProfileId: "00000000-0000-0000-0000-000000000000",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			respErr: "rpc error: code = NotFound desc = Not found",
		},
		{
			name: "guest: show profile event by UUID",
			req: &eventpb.Event_Request{
				Id: &eventpb.Event_Request_ProfileId{
					ProfileId: "59fab0fa-8f0a-4065-8863-0dae40166015",
				},
				UserId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
			},
			respErr: "rpc error: code = NotFound desc = Not found",
		},

		// project event
		{
			name: "owner: show project event by UUID",
			req: &eventpb.Event_Request{
				Id: &eventpb.Event_Request_ProjectId{
					ProjectId: "163dee10-2a74-4436-9507-65a97a711ba8",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &eventpb.Event_Response{
				Ip:       "192.168.0.1",
				Event:    1,
				MetaData: []byte("{}"),
			},
		},
		{
			name: "owner: show project event by fake UUID",
			req: &eventpb.Event_Request{
				Id: &eventpb.Event_Request_ProjectId{
					ProjectId: "00000000-0000-0000-0000-000000000000",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			respErr: "rpc error: code = NotFound desc = Not found",
		},
		{
			name: "guest: show project event by UUID",
			req: &eventpb.Event_Request{
				Id: &eventpb.Event_Request_ProjectId{
					ProjectId: "163dee10-2a74-4436-9507-65a97a711ba8",
				},
				UserId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
			},
			respErr: "rpc error: code = NotFound desc = Not found",
		},

		// server event
		{
			name: "owner: show server event by UUID",
			req: &eventpb.Event_Request{
				Id: &eventpb.Event_Request_ServerId{
					ServerId: "0b1df8d7-c0cd-4a48-bcfc-248b2abe0c93",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &eventpb.Event_Response{
				Ip:       "2001:db8:a0b:12f0::1",
				Event:    8,
				MetaData: []byte("{}"),
			},
		},
		{
			name: "owner: show server event by fake UUID",
			req: &eventpb.Event_Request{
				Id: &eventpb.Event_Request_ServerId{
					ServerId: "00000000-0000-0000-0000-000000000000",
				},
				UserId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			respErr: "rpc error: code = NotFound desc = Not found",
		},
		{
			name: "guest: show server event by UUID",
			req: &eventpb.Event_Request{
				Id: &eventpb.Event_Request_ServerId{
					ServerId: "0b1df8d7-c0cd-4a48-bcfc-248b2abe0c93",
				},
				UserId: "c180ad5c-0c65-4cee-8725-12931cb5abb3",
			},
			respErr: "rpc error: code = NotFound desc = Not found",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			a := eventpb.NewEventHandlersClient(setup.grpc)
			response, err := a.Event(setup.ctx, tt.req)
			if err != nil {
				assert.EqualError(t, err, tt.respErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.resp.GetIp(), response.GetIp())
			assert.Equal(t, tt.resp.GetEvent(), response.GetEvent())
			assert.Equal(t, tt.resp.GetMetaData(), response.GetMetaData())
		})
	}
}

func Test_AddEvent(t *testing.T) {
	//t.Parallel()
	setup, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testCases := []struct {
		name    string
		req     *eventpb.AddEvent_Request
		respErr string
	}{
		{
			name:    "request without parameters",
			req:     &eventpb.AddEvent_Request{},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},

		// profile
		{
			name: "add profile event",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ProfileId{
					ProfileId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				},
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     eventpb.Type_onActive,
				MetaData:  []byte("{\"test\":1}"),
			},
		},

		{
			name: "add profile event with the fake profile UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ProfileId{
					ProfileId: "00000000-0000-0000-0000-000000000000",
				},
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     eventpb.Type_onActive,
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},

		{
			name: "error add profile log with incorrect profile UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ProfileId{
					ProfileId: "abc",
				},
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     eventpb.Type_onActive,
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},

		// project
		{
			name: "add project event",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ProjectId{
					ProjectId: "26060c68-5a06-4a57-b87a-be0f1e787157",
				},
				UserId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     eventpb.Type_onActive,
				MetaData:  []byte("{}"),
			},
		},
		{
			name: "add profile event with out user UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ProjectId{
					ProjectId: "26060c68-5a06-4a57-b87a-be0f1e787157",
				},
				Ip:       "192.168.1.1",
				Event:    eventpb.Type_onActive,
				MetaData: []byte("{}"),
			},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},
		{
			name: "add profile event with the fake profile UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ProjectId{
					ProjectId: "00000000-0000-0000-0000-000000000000",
				},
				UserId:   "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				Ip:       "192.168.1.1",
				Event:    eventpb.Type_onActive,
				MetaData: []byte("{}"),
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
		{
			name: "add profile event with the incorrect profile UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ProjectId{
					ProjectId: "abc",
				},
				UserId:   "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				Ip:       "192.168.1.1",
				Event:    eventpb.Type_onActive,
				MetaData: []byte("{}"),
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
		{
			name: "add profile event with the fake user UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ProjectId{
					ProjectId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				},
				UserId:   "00000000-0000-0000-0000-000000000000",
				Ip:       "192.168.1.1",
				Event:    eventpb.Type_onActive,
				MetaData: []byte("{}"),
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},

		// server
		{
			name: "add server event",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ServerId{
					ServerId: "0c3a8869-6fc0-4666-bf60-15475473392a",
				},
				UserId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     eventpb.Type_onActive,
				MetaData:  []byte("{}"),
			},
		},
		{
			name: "add server event with out user UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ServerId{
					ServerId: "0c3a8869-6fc0-4666-bf60-15475473392a",
				},
				Ip:       "192.168.1.1",
				Event:    eventpb.Type_onActive,
				MetaData: []byte("{}"),
			},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},
		{
			name: "add server event with the fake server UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ServerId{
					ServerId: "00000000-0000-0000-0000-000000000000",
				},
				UserId:   "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				Ip:       "192.168.1.1",
				Event:    eventpb.Type_onActive,
				MetaData: []byte("{}"),
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
		{
			name: "add server event with the fake server UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ServerId{
					ServerId: "00000000-0000-0000-0000-000000000000",
				},
				UserId:   "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				Ip:       "192.168.1.1",
				Event:    eventpb.Type_onActive,
				MetaData: []byte("{}"),
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
		{
			name: "add profile event with the incorrect profile UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ServerId{
					ServerId: "abc",
				},
				UserId:   "0c3a8869-6fc0-4666-bf60-15475473392a",
				Ip:       "192.168.1.1",
				Event:    eventpb.Type_onActive,
				MetaData: []byte("{}"),
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
		{
			name: "add profile event with the fake user id UUID",
			req: &eventpb.AddEvent_Request{
				Id: &eventpb.AddEvent_Request_ServerId{
					ServerId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				},
				UserId:   "00000000-0000-0000-0000-000000000000",
				Ip:       "192.168.1.1",
				Event:    eventpb.Type_onActive,
				MetaData: []byte("{}"),
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			a := eventpb.NewEventHandlersClient(setup.grpc)
			response, err := a.AddEvent(setup.ctx, tt.req)
			if err != nil {
				assert.EqualError(t, err, tt.respErr)
				return
			}

			assert.NoError(t, err)
			if _, err := uuid.Parse(response.GetRecordId()); err != nil {
				assert.Error(t, err)
			}
		})
	}
}
