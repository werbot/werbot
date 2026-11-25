//go:build cluster || load || redteam

package cluster

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"

	"connectrpc.com/connect"

	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// Enroll registers a new instance with a fresh ed25519 key in the pool of
// token through api, with the "ssh" connector. It returns the enrollment
// answer and the instance key; err is the Connect error of a refusal.
func Enroll(ctx context.Context, api API, token, hostname, version string) (*werbotv1.EnrollResponse, ed25519.PrivateKey, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "werbot-gateway"}}, key)
	if err != nil {
		return nil, nil, err
	}
	res, err := api.Enroll.Enroll(ctx, connect.NewRequest(&werbotv1.EnrollRequest{
		JoinToken: token, CsrDer: csr, Hostname: hostname, Version: version, Connectors: []string{"ssh"},
	}))
	if err != nil {
		return nil, nil, err
	}
	return res.Msg, key, nil
}

// TokenUses returns uses of the join token id of pool.
func (s *Stand) TokenUses(t testing.TB, pool, id string) int32 {
	t.Helper()
	res, err := s.Edge.Gateways.ListJoinTokens(context.Background(), connect.NewRequest(&werbotv1.ListJoinTokensRequest{GatewayId: pool}))
	require.NoError(t, err)
	for _, tok := range res.Msg.GetTokens() {
		if tok.GetId() == id {
			return tok.GetUses()
		}
	}
	require.Failf(t, "token not listed", "id %s", id)
	return 0
}
