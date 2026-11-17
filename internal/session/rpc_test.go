package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/session"
)

// rpcClient serves SessionService with the principal named by the X-User header (none when absent).
func rpcClient(t *testing.T, e recEnv, users map[string]principal.Principal) werbotv1connect.SessionServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewSessionServiceHandler(session.NewSessionHandler(session.New(e.pool), e.rec), connect.WithInterceptors(validate.NewInterceptor())))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := users[r.Header.Get("X-User")]; ok {
			r = r.WithContext(principal.With(r.Context(), p))
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return werbotv1connect.NewSessionServiceClient(srv.Client(), srv.URL)
}

func as[T any](user string, m *T) *connect.Request[T] {
	r := connect.NewRequest(m)
	r.Header().Set("X-User", user)
	return r
}

func TestSessionRPC(t *testing.T) {
	e, _ := finishedEnv(t)
	other := e.f.StartSession(t, e.pool, e.f.OtherMemberID)
	c := rpcClient(t, e, map[string]principal.Principal{
		"member": e.f.Principal(e.f.MemberID), "admin": e.f.Principal(e.f.AdminID)})
	ctx := context.Background()

	res, err := c.ListSessions(ctx, as("member", &werbotv1.ListSessionsRequest{}))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetSessions(), 1)
	s := res.Msg.GetSessions()[0]
	assert.Equal(t, e.sid.String(), s.GetId())
	assert.Equal(t, "web", s.GetResourceName())
	assert.Equal(t, "10.0.0.1", s.GetClientIp())
	assert.True(t, s.GetRecording().GetFinished())
	assert.Equal(t, int32(3), s.GetRecording().GetChunks())
	assert.Nil(t, s.GetEndedAt())

	res, err = c.ListSessions(ctx, as("admin", &werbotv1.ListSessionsRequest{Status: werbotv1.SessionStatus_SESSION_STATUS_ACTIVE}))
	require.NoError(t, err)
	assert.Len(t, res.Msg.GetSessions(), 2)

	_, err = c.ListSessions(ctx, as("member", &werbotv1.ListSessionsRequest{UserId: e.f.OtherMemberID.String()}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = c.ListSessions(ctx, as("admin", &werbotv1.ListSessionsRequest{UserId: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	got, err := c.GetSession(ctx, as("member", &werbotv1.SessionServiceGetSessionRequest{Id: e.sid.String()}))
	require.NoError(t, err)
	assert.Equal(t, e.sid.String(), got.Msg.GetSession().GetId())

	v, err := c.VerifyRecording(ctx, as("member", &werbotv1.VerifyRecordingRequest{SessionId: e.sid.String()}))
	require.NoError(t, err)
	assert.True(t, v.Msg.GetVerified())

	_, err = c.VerifyRecording(ctx, as("member", &werbotv1.VerifyRecordingRequest{SessionId: other.String()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = c.VerifyRecording(ctx, as("member", &werbotv1.VerifyRecordingRequest{SessionId: uuid.NewString()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = c.VerifyRecording(ctx, as("admin", &werbotv1.VerifyRecordingRequest{SessionId: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	v, err = c.VerifyRecording(ctx, as("admin", &werbotv1.VerifyRecordingRequest{SessionId: other.String()}))
	require.NoError(t, err)
	assert.Equal(t, session.ReasonNoRecording, v.Msg.GetReason())

	e.store.down.Store(true)
	_, err = c.VerifyRecording(ctx, as("member", &werbotv1.VerifyRecordingRequest{SessionId: e.sid.String()}))
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))

	_, err = c.ListSessions(ctx, connect.NewRequest(&werbotv1.ListSessionsRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = c.GetSession(ctx, connect.NewRequest(&werbotv1.SessionServiceGetSessionRequest{Id: e.sid.String()}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = c.VerifyRecording(ctx, connect.NewRequest(&werbotv1.VerifyRecordingRequest{SessionId: e.sid.String()}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestSessionRPCValidation(t *testing.T) {
	e, _ := finishedEnv(t)
	c := rpcClient(t, e, map[string]principal.Principal{"admin": e.f.Principal(e.f.AdminID)})
	for name, req := range map[string]*werbotv1.ListSessionsRequest{
		"undefined status": {Status: 99}, "negative page": {Page: -1}, "huge page": {Page: 1 << 30}, "bad user": {UserId: "x"}} {
		_, err := c.ListSessions(t.Context(), as("admin", req))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}
	_, err := c.ListSessions(t.Context(), as("admin", &werbotv1.ListSessionsRequest{PageSize: 500}))
	assert.NoError(t, err, "page size above the max is clamped, not rejected")
}
