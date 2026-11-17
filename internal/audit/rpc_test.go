package audit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/audit"
	"github.com/werbot/werbot/internal/platform/principal"
)

// rpcClient serves AuditService (with the validate interceptor) as the principal named by X-User.
func rpcClient(t *testing.T, e env, users map[string]principal.Principal) werbotv1connect.AuditServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuditServiceHandler(audit.NewHandler(e.svc), connect.WithInterceptors(validate.NewInterceptor())))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := users[r.Header.Get("X-User")]; ok {
			r = r.WithContext(principal.With(r.Context(), p))
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return werbotv1connect.NewAuditServiceClient(srv.Client(), srv.URL)
}

func as[T any](user string, m *T) *connect.Request[T] {
	r := connect.NewRequest(m)
	r.Header().Set("X-User", user)
	return r
}

func TestAuditRPCMemberDenied(t *testing.T) {
	e := newEnv(t)
	id := e.write(t, e.a, &e.a.OwnerID, "member.invite", "member", "m1", time.Now())
	c := rpcClient(t, e, map[string]principal.Principal{
		"member": e.a.Principal(e.a.MemberID), "owner": e.a.Principal(e.a.OwnerID)})
	ctx := t.Context()

	_, err := c.ListEvents(ctx, as("member", &werbotv1.ListEventsRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = c.GetEvent(ctx, as("member", &werbotv1.GetEventRequest{Id: id.String()}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = c.ListEvents(ctx, as("", &werbotv1.ListEventsRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	res, err := c.ListEvents(ctx, as("owner", &werbotv1.ListEventsRequest{Action: "member"}))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetEvents(), 1)
	ev := res.Msg.GetEvents()[0]
	assert.Equal(t, id.String(), ev.GetId())
	assert.Equal(t, e.a.OwnerID.String(), ev.GetActorUserId())
	assert.NotEmpty(t, ev.GetActorLogin())
	assert.Equal(t, "v", ev.GetData().AsMap()["k"])
	assert.Zero(t, res.Msg.GetNextPage())

	got, err := c.GetEvent(ctx, as("owner", &werbotv1.GetEventRequest{Id: id.String()}))
	require.NoError(t, err)
	assert.Equal(t, "m1", got.Msg.GetEvent().GetTargetId())
}

func TestAuditRPCValidation(t *testing.T) {
	e := newEnv(t)
	c := rpcClient(t, e, map[string]principal.Principal{"owner": e.a.Principal(e.a.OwnerID)})
	for name, req := range map[string]*werbotv1.ListEventsRequest{
		"bad actor": {ActorUserId: "nope"}, "negative page": {Page: -1}, "huge page": {Page: 1 << 30}, "negative size": {PageSize: -1}} {
		_, err := c.ListEvents(t.Context(), as("owner", req))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}
	bad := &timestamppb.Timestamp{Seconds: 1 << 50}
	for name, req := range map[string]*werbotv1.ListEventsRequest{"from out of range": {From: bad}, "to out of range": {To: bad},
		"bad nanos": {From: &timestamppb.Timestamp{Nanos: -1}}} {
		_, err := c.ListEvents(t.Context(), as("owner", req))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}
	_, err := c.GetEvent(t.Context(), as("owner", &werbotv1.GetEventRequest{Id: "nope"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = c.ListEvents(t.Context(), as("owner", &werbotv1.ListEventsRequest{PageSize: 100000}))
	assert.NoError(t, err, "page size above the max is clamped, not rejected")
	_, err = c.GetEvent(t.Context(), as("owner", &werbotv1.GetEventRequest{Id: "0197a5c0-0000-7000-8000-000000000000"}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
