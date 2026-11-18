//go:build cluster

package cluster

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

const enrollHostname = "e2e-enroll"

// joinToken creates a join token for pool-a with maxUses and returns its id and secret.
func (s *Stand) joinToken(t testing.TB, maxUses int32) (id, token string) {
	t.Helper()
	res, err := s.Edge.Gateways.CreateJoinToken(context.Background(), connect.NewRequest(&werbotv1.CreateJoinTokenRequest{
		GatewayId: s.PoolID.String(), MaxUses: maxUses,
	}))
	require.NoError(t, err)
	return res.Msg.GetId(), res.Msg.GetToken()
}

// deleteEnrolledInstances removes every pool-a instance named "e2e-enroll".
func (s *Stand) deleteEnrolledInstances(t testing.TB) {
	t.Helper()
	ctx := context.Background() // Cleanup context
	res, err := s.Edge.Gateways.ListGateways(ctx, connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
	require.NoError(t, err)
	for _, g := range res.Msg.GetGateways() {
		for _, in := range g.GetInstances() {
			if in.GetHostname() != enrollHostname {
				continue
			}
			_, err := s.Edge.Gateways.DeleteGatewayInstance(ctx, connect.NewRequest(&werbotv1.DeleteGatewayInstanceRequest{Id: in.GetId()}))
			require.NoError(t, err)
		}
	}
}

// tokenUses returns uses of the pool-a join token id.
func (s *Stand) tokenUses(t testing.TB, id string) int32 {
	t.Helper()
	res, err := s.Edge.Gateways.ListJoinTokens(context.Background(), connect.NewRequest(&werbotv1.ListJoinTokensRequest{GatewayId: s.PoolID.String()}))
	require.NoError(t, err)
	for _, tok := range res.Msg.GetTokens() {
		if tok.GetId() == id {
			return tok.GetUses()
		}
	}
	require.Failf(t, "token not listed", "id %s", id)
	return 0
}

// refusal asserts err is a PermissionDenied Connect error and returns its message.
func refusal(t testing.TB, err error) string {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	return ce.Message()
}

// TestScenario09EnrollRevokedAndExhaustedToken: an exhausted, a revoked and an
// unknown join token are refused alike (same message); one single-use token
// enrolls exactly one of two concurrent callers and its uses stay 1.
func TestScenario09EnrollRevokedAndExhaustedToken(t *testing.T) {
	st.Healthy(t)
	t.Cleanup(func() { st.deleteEnrolledInstances(t) })
	ctx := context.Background()
	enroll := func(api API, token string) error {
		_, _, err := Enroll(ctx, api, token, enrollHostname, "test")
		return err
	}

	_, tok1 := st.joinToken(t, 1)
	require.NoError(t, enroll(st.A, tok1), "first use of a single-use token")
	exhausted := refusal(t, enroll(st.B, tok1))

	id2, tok2 := st.joinToken(t, 5)
	_, err := st.Edge.Gateways.RevokeJoinToken(ctx, connect.NewRequest(&werbotv1.RevokeJoinTokenRequest{Id: id2}))
	require.NoError(t, err)
	revoked := refusal(t, enroll(st.Edge, tok2))

	secret := make([]byte, 32)
	_, err = rand.Read(secret)
	require.NoError(t, err)
	_, fp, ok := strings.Cut(tok2, ".")
	require.True(t, ok, "token carries a fingerprint suffix")
	unknown := refusal(t, enroll(st.Edge, "wjt_"+base64.RawURLEncoding.EncodeToString(secret)+"."+fp))

	assert.Equal(t, exhausted, revoked, "exhausted and revoked differ")
	assert.Equal(t, exhausted, unknown, "exhausted and unknown differ")

	id3, tok3 := st.joinToken(t, 1)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, api := range []API{st.A, st.B} {
		wg.Go(func() { errs[i] = enroll(api, tok3) })
	}
	wg.Wait()
	if errs[0] == nil {
		assert.Equal(t, exhausted, refusal(t, errs[1]))
	} else {
		require.NoError(t, errs[1], "exactly one concurrent enroll must win")
		assert.Equal(t, exhausted, refusal(t, errs[0]))
	}
	assert.EqualValues(t, 1, st.tokenUses(t, id3))
}
