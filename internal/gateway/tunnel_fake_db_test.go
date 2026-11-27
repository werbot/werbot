package gateway

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
)

// writeTunnel is a TunnelService answering GetWriteCredential with cred, or err.
type writeTunnel struct {
	gatewayv1connect.UnimplementedTunnelServiceHandler
	cred *gatewayv1.Credential
	err  error
	got  *gatewayv1.GetWriteCredentialRequest
}

func (f *writeTunnel) GetWriteCredential(_ context.Context, r *connect.Request[gatewayv1.GetWriteCredentialRequest]) (*connect.Response[gatewayv1.GetWriteCredentialResponse], error) {
	f.got = r.Msg
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(&gatewayv1.GetWriteCredentialResponse{Credential: f.cred}), nil
}

// GetWriteCredential of the plain fake: a server without it.
func (*fakeControl) GetWriteCredential(context.Context, uuid.UUID, string, [32]byte) (Secret, error) {
	return Secret{}, connect.NewError(connect.CodeUnimplemented, nil)
}

func TestGetWriteCredentialCall(t *testing.T) {
	sum, approval := sha256.Sum256([]byte("update t set a = 1")), uuid.NewString()
	f := &writeTunnel{cred: &gatewayv1.Credential{Kind: "password", Value: []byte("pw")}}
	c := newWriteTunnel(t, f)
	sec, err := c.GetWriteCredential(t.Context(), testSessionID, approval, sum)
	require.NoError(t, err)
	assert.Equal(t, Secret{Kind: "password", Value: []byte("pw")}, sec)
	assert.True(t, proto.Equal(&gatewayv1.GetWriteCredentialRequest{SessionId: testSessionID.String(), ApprovalId: approval, OpSha256: sum[:]}, f.got))

	f.cred = nil
	_, err = c.GetWriteCredential(t.Context(), testSessionID, approval, sum)
	require.Error(t, err, "an answer without a credential")
	f.err = connect.NewError(connect.CodePermissionDenied, errors.New("write not approved"))
	_, err = c.GetWriteCredential(t.Context(), testSessionID, approval, sum)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

// newWriteTunnel serves f and returns a tunnel client of it.
func newWriteTunnel(t *testing.T, f *writeTunnel) *tunnelClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(gatewayv1connect.NewTunnelServiceHandler(f))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := &tunnelClient{callTimeout: time.Second}
	c.conn.Store(&tunnelConn{rpc: gatewayv1connect.NewTunnelServiceClient(srv.Client(), srv.URL)})
	return c
}
