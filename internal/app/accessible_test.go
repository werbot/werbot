package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/principal"
)

// ListAccessibleResources runs the real policy: only bound resources of the caller's org are listed.
func TestListAccessibleResourcesUsesPolicy(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, _, owner := s.org(t, "owner")
	bound, err := s.echo(ctx, t, wrapperspb.String("a"), nil)
	require.NoError(t, err)
	_, err = s.echo(ctx, t, wrapperspb.String("b"), nil)
	require.NoError(t, err)
	s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: owner.String(), ResourceId: bound.GetId()})

	otherCtx, _, other := s.org(t, "owner")
	foreign, err := s.echo(otherCtx, t, wrapperspb.String("c"), nil)
	require.NoError(t, err)
	s.bind(otherCtx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: other.String(), ResourceId: foreign.GetId()})

	res := s.accessible(ctx, t)
	require.Len(t, res, 1)
	assert.Equal(t, bound.GetId(), res[0].GetId())
	assert.Equal(t, []string{"stream"}, res[0].GetKinds())
}

// accessible calls ListAccessibleResources as a request from 10.0.0.1 would.
func (s stage4) accessible(ctx context.Context, t *testing.T) []*werbotv1.AccessibleResource {
	t.Helper()
	// a request always carries the client address (clientip.Middleware)
	var res *connect.Response[werbotv1.ListAccessibleResourcesResponse]
	var err error
	r := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	r.RemoteAddr = "10.0.0.1:4000"
	clientip.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		res, err = s.res.ListAccessibleResources(r.Context(), connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	})).ServeHTTP(httptest.NewRecorder(), r)
	require.NoError(t, err)
	return res.Msg.GetResources()
}

// The real access service marks resources an approval policy covers requestable:
// r1 has a policy, r2 a binding, r3 neither.
func TestAccessibleResourcesRequestable(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, _, owner := s.org(t, "owner")
	var ids [3]string
	for i, name := range []string{"r1", "r2", "r3"} {
		r, err := s.echo(ctx, t, wrapperspb.String(name), nil)
		require.NoError(t, err)
		ids[i] = r.GetId()
	}
	s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: owner.String(), ResourceId: ids[1]})
	p, err := principal.Require(ctx)
	require.NoError(t, err)
	_, err = s.d.Access.CreateApprovalPolicy(ctx, p, access.ApprovalPolicy{Name: "r1", Mode: access.ModeManual,
		Selector: access.Selector{ResourceIDs: []uuid.UUID{uuid.MustParse(ids[0])}}, Requesters: access.Subjects{AllUsers: true}})
	require.NoError(t, err)

	got := map[string][2]bool{}
	for _, r := range s.accessible(ctx, t) {
		got[r.GetId()] = [2]bool{r.GetAllowed(), r.GetRequestable()}
	}
	assert.Equal(t, map[string][2]bool{ids[0]: {false, true}, ids[1]: {true, false}}, got)
}
