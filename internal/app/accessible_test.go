package app

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/clientip"
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

	// a request always carries the client address (clientip.Middleware)
	var res *connect.Response[werbotv1.ListAccessibleResourcesResponse]
	r := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	r.RemoteAddr = "10.0.0.1:4000"
	clientip.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		res, err = s.res.ListAccessibleResources(r.Context(), connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	})).ServeHTTP(httptest.NewRecorder(), r)
	require.NoError(t, err)
	require.Len(t, res.Msg.GetResources(), 1)
	assert.Equal(t, bound.GetId(), res.Msg.GetResources()[0].GetId())
	assert.Equal(t, []string{"stream"}, res.Msg.GetResources()[0].GetKinds())
}
