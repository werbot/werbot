package inventory_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	validatepb "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

const tunnelURL = "https://tunnel.example:8443"

type gwRPCEnv struct {
	gwEnv
	srv      *httptest.Server
	admin    werbotv1connect.GatewayServiceClient // owner
	member   werbotv1connect.GatewayServiceClient
	enroller werbotv1connect.GatewayEnrollServiceClient // no principal
}

// gwRPCSetup serves both services as serve does, except the principal comes
// from the X-Test-Role header instead of a session cookie.
func gwRPCSetup(t *testing.T) gwRPCEnv {
	t.Helper()
	e := gwSetup(t)
	memberID := dbtest.User(t, e.pool, e.org, "member")
	rpc := inventory.NewGatewayRPC(e.gw, tunnelURL)
	opts := connect.WithInterceptors(validate.NewInterceptor())
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewGatewayServiceHandler(rpc, opts))
	mux.Handle(werbotv1connect.NewGatewayEnrollServiceHandler(rpc, opts))
	asPrincipal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-Test-Role") {
		case "owner":
			r = r.WithContext(principal.With(r.Context(), principal.Principal{UserID: e.owner, OrgID: e.org, Role: "owner"}))
		case "member":
			r = r.WithContext(principal.With(r.Context(), principal.Principal{UserID: memberID, OrgID: e.org, Role: "member"}))
		}
		mux.ServeHTTP(w, r)
	})
	srv := httptest.NewServer(clientip.Middleware(nil)(asPrincipal))
	t.Cleanup(srv.Close)
	as := func(role string) connect.ClientOption {
		return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set("X-Test-Role", role)
				return next(ctx, req)
			}
		}))
	}
	return gwRPCEnv{gwEnv: e, srv: srv,
		admin:    werbotv1connect.NewGatewayServiceClient(srv.Client(), srv.URL, as("owner")),
		member:   werbotv1connect.NewGatewayServiceClient(srv.Client(), srv.URL, as("member")),
		enroller: werbotv1connect.NewGatewayEnrollServiceClient(srv.Client(), srv.URL)}
}

func (e gwRPCEnv) enrollMsg(t *testing.T, tok string) *werbotv1.EnrollRequest {
	t.Helper()
	return &werbotv1.EnrollRequest{JoinToken: tok, CsrDer: csr(t), Hostname: "gw-1", Version: "v0.1.0", Connectors: []string{"ssh"}}
}

func TestGatewayRPCRoundTrip(t *testing.T) {
	e := gwRPCSetup(t)
	ctx := context.Background()

	created, err := e.admin.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "edge"}))
	require.NoError(t, err)
	gwID := created.Msg.GetGateway().GetId()
	list, err := e.admin.ListGateways(ctx, connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetGateways(), 1)
	assert.Equal(t, "edge", list.Msg.GetGateways()[0].GetName())
	assert.Empty(t, list.Msg.GetGateways()[0].GetInstances())

	tok, err := e.admin.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{GatewayId: gwID}))
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), tok.Msg.GetExpiresAt().AsTime(), time.Minute)
	toks, err := e.admin.ListJoinTokens(ctx, connect.NewRequest(&werbotv1.ListJoinTokensRequest{GatewayId: gwID}))
	require.NoError(t, err)
	require.Len(t, toks.Msg.GetTokens(), 1)
	assert.EqualValues(t, 10, toks.Msg.GetTokens()[0].GetMaxUses())
	assert.EqualValues(t, 0, toks.Msg.GetTokens()[0].GetUses())

	en, err := e.enroller.Enroll(ctx, connect.NewRequest(e.enrollMsg(t, tok.Msg.GetToken())))
	require.NoError(t, err)
	assert.Equal(t, tunnelURL, en.Msg.GetTunnelUrl())
	assert.True(t, strings.HasPrefix(en.Msg.GetHostKeyPem(), "-----BEGIN OPENSSH PRIVATE KEY-----"))
	assert.NotEmpty(t, en.Msg.GetCertPem())
	assert.NotEmpty(t, en.Msg.GetCaPem())
	list, err = e.admin.ListGateways(ctx, connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
	require.NoError(t, err)
	ins := list.Msg.GetGateways()[0].GetInstances()
	require.Len(t, ins, 1)
	assert.Equal(t, en.Msg.GetInstanceId(), ins[0].GetId())
	assert.Equal(t, "gw-1", ins[0].GetHostname())
	assert.Equal(t, "127.0.0.1", ins[0].GetClientIp())

	_, err = e.admin.DeleteGatewayInstance(ctx, connect.NewRequest(&werbotv1.DeleteGatewayInstanceRequest{Id: en.Msg.GetInstanceId()}))
	require.NoError(t, err)
	list, err = e.admin.ListGateways(ctx, connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
	require.NoError(t, err)
	assert.Empty(t, list.Msg.GetGateways()[0].GetInstances())

	_, err = e.admin.RevokeJoinToken(ctx, connect.NewRequest(&werbotv1.RevokeJoinTokenRequest{Id: tok.Msg.GetId()}))
	require.NoError(t, err)
	toks, err = e.admin.ListJoinTokens(ctx, connect.NewRequest(&werbotv1.ListJoinTokensRequest{GatewayId: gwID}))
	require.NoError(t, err)
	assert.NotNil(t, toks.Msg.GetTokens()[0].GetRevokedAt())

	_, err = e.admin.DeleteGateway(ctx, connect.NewRequest(&werbotv1.DeleteGatewayRequest{Id: gwID}))
	require.NoError(t, err)
}

func TestGatewayRPCErrors(t *testing.T) {
	e := gwRPCSetup(t)
	ctx := context.Background()
	g := e.newPool(t, "edge")
	gwID := g.ID.String()

	_, err := e.member.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "x"}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "member")
	_, err = e.member.ListGateways(ctx, connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "member list")

	_, err = e.admin.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{
		GatewayId: gwID, ExpiresIn: durationpb.New(30 * time.Minute)}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "expires_in 30m")
	for _, d := range []time.Duration{8761 * time.Hour, -time.Hour} {
		_, err = e.admin.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{
			GatewayId: gwID, ExpiresIn: durationpb.New(d)}))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "expires_in %v", d)
		assert.Equal(t, []string{"expires_in"}, violationFields(err), "rejected by the proto rule, %v", d)
	}
	_, err = e.admin.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{GatewayId: gwID, MaxUses: 1001}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "max_uses 1001")
	_, err = e.admin.DeleteGateway(ctx, connect.NewRequest(&werbotv1.DeleteGatewayRequest{Id: "xyz"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "bad id")

	one, err := e.admin.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{GatewayId: gwID, MaxUses: 1}))
	require.NoError(t, err)
	_, err = e.enroller.Enroll(ctx, connect.NewRequest(e.enrollMsg(t, one.Msg.GetToken())))
	require.NoError(t, err)
	_, err = e.enroller.Enroll(ctx, connect.NewRequest(e.enrollMsg(t, one.Msg.GetToken())))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "exhausted token")

	long := e.enrollMsg(t, one.Msg.GetToken())
	long.Version = strings.Repeat("v", 65)
	_, err = e.enroller.Enroll(ctx, connect.NewRequest(long))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "version over 64")
	many := e.enrollMsg(t, one.Msg.GetToken())
	many.Connectors = make([]string, 65)
	for i := range many.Connectors {
		many.Connectors[i] = "ssh"
	}
	_, err = e.enroller.Enroll(ctx, connect.NewRequest(many))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "connectors over 64")

	fresh, err := e.admin.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{GatewayId: gwID, MaxUses: 1}))
	require.NoError(t, err)
	long = e.enrollMsg(t, fresh.Msg.GetToken())
	long.Version = strings.Repeat("v", 65)
	_, err = e.enroller.Enroll(ctx, connect.NewRequest(long))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "version over 64, fresh token")
	_, err = e.enroller.Enroll(ctx, connect.NewRequest(e.enrollMsg(t, fresh.Msg.GetToken())))
	require.NoError(t, err, "the rejected request did not spend the single use")

	e.create(t, "res", nil)
	_, err = e.admin.DeleteGateway(ctx, connect.NewRequest(&werbotv1.DeleteGatewayRequest{Id: gwID}))
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err), "pool with resources")
}

// violationFields returns the field paths of the protovalidate violations in err.
func violationFields(err error) []string {
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok {
		return nil
	}
	var out []string
	for _, d := range ce.Details() {
		v, err := d.Value()
		if vs, ok := v.(*validatepb.Violations); ok && err == nil {
			for _, x := range vs.GetViolations() {
				out = append(out, protovalidate.FieldPathString(x.GetField()))
			}
		}
	}
	return out
}
