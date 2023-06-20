package logging_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	loggingpb "github.com/werbot/werbot/internal/grpc/logging/proto"
	"github.com/werbot/werbot/internal/utils/test"
)

type testSetup struct {
	ctx  context.Context
	grpc *grpc.ClientConn
}

func setupTest(t *testing.T) (testSetup, func(t *testing.T)) {
	ctx := context.Background()

	postgres, err := test.CreateDB(t, "../../../migration", "../../../fixtures/migration")
	grpc := test.CreateGRPC(ctx, t, &test.Service{DB: postgres.Conn})

	if err != nil {
		t.Error(err)
	}

	return testSetup{
			ctx:  ctx,
			grpc: grpc,
		}, func(t *testing.T) {
			postgres.Stop(t)
			grpc.Close()
		}
}

func Test_logging_ListRecords(t *testing.T) {
	//t.Parallel()
	setup, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testCases := []struct {
		name    string
		req     *loggingpb.ListRecords_Request
		resp    *loggingpb.ListRecords_Response
		respErr string
	}{
		{
			name: "error show record by incorrect logger",
			req: &loggingpb.ListRecords_Request{
				Logger: loggingpb.Logger_logger_unspecified,
			},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},
		{
			name: "success show record by profile_id",
			req: &loggingpb.ListRecords_Request{
				Logger:   loggingpb.Logger_profile,
				RecordId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			},
			resp: &loggingpb.ListRecords_Response{
				Total: 2,
			},
		},
		{
			name: "error record by fake profile_id",
			req: &loggingpb.ListRecords_Request{
				Logger:   loggingpb.Logger_profile,
				RecordId: "008feb1d-12f2-4bc3-97ff-c8d7fb9f7600",
			},
			resp: &loggingpb.ListRecords_Response{},
		},
		{
			name: "success show record by project_id",
			req: &loggingpb.ListRecords_Request{
				Logger:   loggingpb.Logger_project,
				RecordId: "26060c68-5a06-4a57-b87a-be0f1e787157",
			},
			resp: &loggingpb.ListRecords_Response{
				Total: 2,
			},
		},
		{
			name: "success show record by server_id",
			req: &loggingpb.ListRecords_Request{
				Logger:   loggingpb.Logger_server,
				RecordId: "0c3a8869-6fc0-4666-bf60-15475473392a",
			},
			resp: &loggingpb.ListRecords_Response{
				Total: 11,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			a := loggingpb.NewLoggingHandlersClient(setup.grpc)
			response, err := a.ListRecords(setup.ctx, tt.req)
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.resp.GetTotal(), response.GetTotal())
		})
	}
}

func Test_logging_Record(t *testing.T) {
	//t.Parallel()
	setup, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testCases := []struct {
		name    string
		req     *loggingpb.Record_Request
		resp    *loggingpb.Record_Response
		respErr string
	}{
		{
			name: "error show profile record by incorrect logger",
			req: &loggingpb.Record_Request{
				Logger: loggingpb.Logger_logger_unspecified,
			},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},
		{
			name: "success show profile record by id",
			req: &loggingpb.Record_Request{
				Logger:   loggingpb.Logger_profile,
				RecordId: "59fab0fa-8f0a-4065-8863-0dae40166015",
			},
			resp: &loggingpb.Record_Response{
				UserId:   "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				Event:    1,
				MetaData: []byte("{}"),
			},
		},
		{
			name: "error show profile record by fake id",
			req: &loggingpb.Record_Request{
				Logger:   loggingpb.Logger_profile,
				RecordId: "59fab0fa-8f0a-4065-8863-0dae40166000",
			},
			respErr: "rpc error: code = NotFound desc = Not found",
		},
		{
			name: "success show project record by id",
			req: &loggingpb.Record_Request{
				Logger:   loggingpb.Logger_project,
				RecordId: "163dee10-2a74-4436-9507-65a97a711ba8",
			},
			resp: &loggingpb.Record_Response{
				UserId:   "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				Event:    1,
				MetaData: []byte("{}"),
			},
		},
		{
			name: "success show server record by id",
			req: &loggingpb.Record_Request{
				Logger:   loggingpb.Logger_server,
				RecordId: "0b1df8d7-c0cd-4a48-bcfc-248b2abe0c93",
			},
			resp: &loggingpb.Record_Response{
				UserId:   "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				Event:    8,
				MetaData: []byte("{}"),
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			a := loggingpb.NewLoggingHandlersClient(setup.grpc)
			response, err := a.Record(setup.ctx, tt.req)
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.resp.GetUserId(), response.GetUserId())
			require.Equal(t, tt.resp.GetEvent(), response.GetEvent())
			require.Equal(t, tt.resp.GetMetaData(), response.GetMetaData())
		})
	}
}

func Test_logging_AddRecord(t *testing.T) {
	//t.Parallel()
	setup, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testCases := []struct {
		name    string
		req     *loggingpb.AddRecord_Request
		respErr string
	}{
		{
			name: "error add log",
			req: &loggingpb.AddRecord_Request{
				Logger: loggingpb.Logger_logger_unspecified,
			},
			respErr: "rpc error: code = InvalidArgument desc = Invalid argument",
		},
		{
			name: "success add profile log",
			req: &loggingpb.AddRecord_Request{
				Logger:    loggingpb.Logger_profile,
				Id:        "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     loggingpb.EventType_onActive,
			},
		},
		{
			name: "error add profile log with incorrect profile_id",
			req: &loggingpb.AddRecord_Request{
				Logger:    loggingpb.Logger_profile,
				Id:        "008feb1d-12f2-4bc3-97ff-c8d7fb9f7600",
				UserId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     loggingpb.EventType_onActive,
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
		{
			name: "error add profile log with incorrect user_id",
			req: &loggingpb.AddRecord_Request{
				Logger:    loggingpb.Logger_profile,
				Id:        "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7600",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     loggingpb.EventType_onActive,
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
		{
			name: "success add project log",
			req: &loggingpb.AddRecord_Request{
				Logger:    loggingpb.Logger_project,
				Id:        "26060c68-5a06-4a57-b87a-be0f1e787157",
				UserId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     loggingpb.EventType_onActive,
			},
		},
		{
			name: "error add project log",
			req: &loggingpb.AddRecord_Request{
				Logger:    loggingpb.Logger_project,
				Id:        "26060c68-5a06-4a57-b87a-be0f1e787100",
				UserId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     loggingpb.EventType_onActive,
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
		{
			name: "success add server log",
			req: &loggingpb.AddRecord_Request{
				Logger:    loggingpb.Logger_server,
				Id:        "0c3a8869-6fc0-4666-bf60-15475473392a",
				UserId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     loggingpb.EventType_onActive,
			},
		},
		{
			name: "error add server log",
			req: &loggingpb.AddRecord_Request{
				Logger:    loggingpb.Logger_server,
				Id:        "0c3a8869-6fc0-4666-bf60-154754733000",
				UserId:    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.106 Safari/537.36 OPR/38.0.2220.41",
				Ip:        "192.168.1.1",
				Event:     loggingpb.EventType_onActive,
			},
			respErr: "rpc error: code = Aborted desc = Failed to add",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			a := loggingpb.NewLoggingHandlersClient(setup.grpc)
			response, err := a.AddRecord(setup.ctx, tt.req)
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, response.RecordId)
		})
	}
}
