package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// fakeRequests is a fake AccessRequestService; each test sets the funcs it needs.
type fakeRequests struct {
	werbotv1connect.UnimplementedAccessRequestServiceHandler
	create  func(*werbotv1.CreateRequestRequest) (*werbotv1.AccessRequest, error)
	get     func(string) (*werbotv1.AccessRequest, error)
	list    func(*werbotv1.ListRequestsRequest) []*werbotv1.AccessRequest
	cancel  func(string)
	approve func(*werbotv1.ApproveRequest) error
	deny    func(*werbotv1.DenyRequest) error
}

func (f *fakeRequests) CreateRequest(_ context.Context, r *connect.Request[werbotv1.CreateRequestRequest]) (*connect.Response[werbotv1.CreateRequestResponse], error) {
	ar, err := f.create(r.Msg)
	return connect.NewResponse(&werbotv1.CreateRequestResponse{Request: ar}), err
}

func (f *fakeRequests) GetRequest(_ context.Context, r *connect.Request[werbotv1.GetRequestRequest]) (*connect.Response[werbotv1.GetRequestResponse], error) {
	ar, err := f.get(r.Msg.GetId())
	return connect.NewResponse(&werbotv1.GetRequestResponse{Request: ar}), err
}

func (f *fakeRequests) ListRequests(_ context.Context, r *connect.Request[werbotv1.ListRequestsRequest]) (*connect.Response[werbotv1.ListRequestsResponse], error) {
	return connect.NewResponse(&werbotv1.ListRequestsResponse{Requests: f.list(r.Msg)}), nil
}

func (f *fakeRequests) Cancel(_ context.Context, r *connect.Request[werbotv1.CancelRequest]) (*connect.Response[werbotv1.CancelResponse], error) {
	f.cancel(r.Msg.GetId())
	return connect.NewResponse(&werbotv1.CancelResponse{}), nil
}

func (f *fakeRequests) Approve(_ context.Context, r *connect.Request[werbotv1.ApproveRequest]) (*connect.Response[werbotv1.ApproveResponse], error) {
	return connect.NewResponse(&werbotv1.ApproveResponse{}), f.approve(r.Msg)
}

func (f *fakeRequests) Deny(_ context.Context, r *connect.Request[werbotv1.DenyRequest]) (*connect.Response[werbotv1.DenyResponse], error) {
	return connect.NewResponse(&werbotv1.DenyResponse{}), f.deny(r.Msg)
}

// requestHarness is a harness whose only resource is the requestable "db".
func requestHarness(t *testing.T) (*harness, *fakeRequests) {
	t.Helper()
	h := newHarness(t)
	h.resources[0].Allowed, h.resources[0].Requestable = false, true
	f := &fakeRequests{}
	h.mux.Handle(werbotv1connect.NewAccessRequestServiceHandler(f))
	old := requestPollInterval
	requestPollInterval = time.Millisecond
	t.Cleanup(func() { requestPollInterval = old })
	return h, f
}

func TestRequestApprovedAfterPoll(t *testing.T) {
	h, f := requestHarness(t)
	until := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	var created *werbotv1.CreateRequestRequest
	f.create = func(r *werbotv1.CreateRequestRequest) (*werbotv1.AccessRequest, error) {
		created = r
		return &werbotv1.AccessRequest{Id: "r1", Status: "pending", Url: "https://w.example/requests?id=r1"}, nil
	}
	polls := 0
	f.get = func(string) (*werbotv1.AccessRequest, error) {
		if polls++; polls < 2 {
			return &werbotv1.AccessRequest{Id: "r1", Status: "pending"}, nil
		}
		return &werbotv1.AccessRequest{Id: "r1", Status: "approved", ExpiresAt: timestamppb.New(until)}, nil
	}
	require.NoError(t, h.run("request", "db", "--for", "2h", "--reason", "deploy"))
	assert.Equal(t, h.resources[0].GetId(), created.GetResourceId())
	assert.Equal(t, 2*time.Hour, created.GetDuration().AsDuration())
	assert.Equal(t, "deploy", created.GetReason())
	assert.Contains(t, h.out.String(), "Request r1 is waiting for approval: https://w.example/requests?id=r1")
	assert.Contains(t, h.out.String(), "Access to db granted until "+until.Format(time.RFC3339))
}

func TestRequestAutoApproved(t *testing.T) {
	h, f := requestHarness(t)
	until := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	f.create = func(r *werbotv1.CreateRequestRequest) (*werbotv1.AccessRequest, error) {
		assert.Nil(t, r.GetDuration(), "no --for: the organization default")
		return &werbotv1.AccessRequest{Id: "r1", Status: "approved", ExpiresAt: timestamppb.New(until)}, nil
	}
	require.NoError(t, h.run("request", "db"))
	assert.Equal(t, "Access to db granted until "+until.Format(time.RFC3339)+"\n", h.out.String())
}

func TestRequestDeniedAndExpired(t *testing.T) {
	h, f := requestHarness(t)
	f.create = func(*werbotv1.CreateRequestRequest) (*werbotv1.AccessRequest, error) {
		return &werbotv1.AccessRequest{Id: "r1", Status: "pending"}, nil
	}
	f.get = func(string) (*werbotv1.AccessRequest, error) {
		return &werbotv1.AccessRequest{Id: "r1", Status: "denied", Comment: "no"}, nil
	}
	assert.EqualError(t, h.run("request", "db"), "Request r1 was denied: no")
	f.get = func(string) (*werbotv1.AccessRequest, error) {
		return &werbotv1.AccessRequest{Id: "r1", Status: "expired"}, nil
	}
	assert.EqualError(t, h.run("request", "db"), "Request r1 expired")
}

func TestRequestCtrlCCancels(t *testing.T) {
	h, f := requestHarness(t)
	f.create = func(*werbotv1.CreateRequestRequest) (*werbotv1.AccessRequest, error) {
		return &werbotv1.AccessRequest{Id: "r1", Status: "pending"}, nil
	}
	f.get = func(string) (*werbotv1.AccessRequest, error) {
		h.cancelRunning()
		return &werbotv1.AccessRequest{Id: "r1", Status: "pending"}, nil
	}
	var canceled string
	f.cancel = func(id string) { canceled = id }
	start := time.Now()
	require.NoError(t, h.run("request", "db"))
	assert.Equal(t, "r1", canceled)
	assert.Contains(t, h.out.String(), "Request r1 canceled")
	assert.Less(t, time.Since(start), time.Second)
}

func TestRequestBreakGlassNeedsReason(t *testing.T) {
	h, f := requestHarness(t)
	f.create = func(*werbotv1.CreateRequestRequest) (*werbotv1.AccessRequest, error) {
		t.Error("CreateRequest must not be called")
		return nil, errors.New("unexpected")
	}
	assert.EqualError(t, h.run("request", "db", "--break-glass"), "--break-glass needs --reason")
}

func TestRequestNotRequestable(t *testing.T) {
	h, f := requestHarness(t)
	h.resources[0].Requestable = false
	f.create = func(*werbotv1.CreateRequestRequest) (*werbotv1.AccessRequest, error) {
		t.Error("CreateRequest must not be called")
		return nil, errors.New("unexpected")
	}
	assert.EqualError(t, h.run("request", "db"), "resource db is not requestable")
	assert.EqualError(t, h.run("request", "nope"), "resource nope is not requestable")
}

func TestRequestsTable(t *testing.T) {
	h, f := requestHarness(t)
	var scope werbotv1.RequestScope
	f.list = func(r *werbotv1.ListRequestsRequest) []*werbotv1.AccessRequest {
		scope = r.GetScope()
		at := timestamppb.New(time.Date(2026, 11, 20, 15, 30, 0, 0, time.UTC))
		return []*werbotv1.AccessRequest{
			{Id: "r1", Kind: "access", Status: "pending", ResourceName: "db", RequesterLogin: "bob",
				Duration: durationpb.New(2 * time.Hour), Reason: "deploy"},
			{Id: "r2", Kind: "access", Status: "pending", ExpiresAt: at},
			{Id: "r3", Kind: "access", Status: "approved", ExpiresAt: at},
			{Id: "r4", Kind: "access", Status: "denied", ExpiresAt: at},
			{Id: "r5", Kind: "access", Status: "expired", ExpiresAt: at},
			{Id: "r6", Kind: "command", Status: "approved", ExpiresAt: at},
		}
	}
	require.NoError(t, h.run("requests", "--to-approve"))
	assert.Equal(t, werbotv1.RequestScope_REQUEST_SCOPE_TO_APPROVE, scope)
	assert.Regexp(t, `ID\s+STATUS\s+RESOURCE\s+REQUESTER\s+DURATION\s+EXPIRES\s+REASON\nr1\s+pending\s+db\s+bob\s+2h0m0s\s+-\s+deploy\n`, h.out.String())
	for id, want := range map[string]string{"r2": "2026-11-20T15:30:00Z", "r3": "2026-11-20T15:30:00Z", "r4": "-", "r5": "-", "r6": "-"} {
		assert.Regexp(t, `\n`+id+`\s+\w+\s+0s\s+`+want+`\s*\n`, h.out.String(), "EXPIRES is the deadline of a pending request and the grant end of an approved access request")
	}
}

func TestApproveDeny(t *testing.T) {
	h, f := requestHarness(t)
	var got *werbotv1.ApproveRequest
	f.approve = func(r *werbotv1.ApproveRequest) error { got = r; return nil }
	f.deny = func(*werbotv1.DenyRequest) error {
		return connect.NewError(connect.CodePermissionDenied, errors.New("you cannot decide this request"))
	}
	require.NoError(t, h.run("approve", "r1", "--comment", "ok"))
	assert.Equal(t, "r1", got.GetId())
	assert.Equal(t, "ok", got.GetComment())
	assert.Equal(t, "Request r1 approved\n", h.out.String())
	assert.EqualError(t, h.run("deny", "r1"), "you cannot decide this request")
}

func TestSSHConfigSkipsRequestable(t *testing.T) {
	h := newHarness(t)
	h.resources = append(h.resources, &werbotv1.AccessibleResource{
		Id: "p", Name: "prod", Type: "ssh", Kinds: []string{"terminal"}, SshAddress: "gw.example.com:2222", Requestable: true,
	})
	require.NoError(t, h.run("ssh-config"))
	assert.Contains(t, h.out.String(), "Host db.")
	assert.NotContains(t, h.out.String(), "prod")

	h.out.Reset()
	require.NoError(t, h.run("ls"))
	assert.Regexp(t, `ACCESS\ndb .* yes\nprod .* request\n`, h.out.String())

	assert.EqualError(t, h.run("ssh", "prod"), "resource prod is not accessible; run werbot request prod")
	assert.EqualError(t, h.run("connect", "prod"), "resource prod not found or not accessible")
}
