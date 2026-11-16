package inventory_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

type gwEnv struct {
	env
	gw  *inventory.Gateways
	ca  *tlsca.CA
	box *secretbox.Box
}

func gwSetup(t *testing.T) gwEnv {
	t.Helper()
	e := setup(t)
	box, err := secretbox.New(bytes.Repeat([]byte{7}, 32), nil)
	require.NoError(t, err)
	ca, err := tlsca.LoadOrCreate(context.Background(), tlsca.PostgresStore(e.pool), box)
	require.NoError(t, err)
	return gwEnv{e, inventory.NewGateways(e.pool, ca, box), ca, box}
}

// otherOrg returns an owner context of a new organization.
func (e gwEnv) otherOrg(t *testing.T) (context.Context, uuid.UUID) {
	t.Helper()
	org := dbtest.Org(t, e.pool)
	user := dbtest.User(t, e.pool, org, "owner")
	return principal.With(context.Background(), principal.Principal{UserID: user, OrgID: org, Role: "owner"}), org
}

func (e gwEnv) newPool(t *testing.T, name string) inventory.Gateway {
	t.Helper()
	g, err := e.gw.Create(e.ctx, name)
	require.NoError(t, err)
	return g
}

func (e gwEnv) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

// poolKey opens the stored host key of gatewayID.
func (e gwEnv) poolKey(t *testing.T, gatewayID uuid.UUID) gossh.Signer {
	t.Helper()
	var ct []byte
	var keyID string
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT host_key_ciphertext, host_key_key_id FROM gateway WHERE id = $1", gatewayID).Scan(&ct, &keyID))
	assert.Equal(t, e.box.KeyID(), keyID)
	plain, err := e.box.Open(ct, keyID)
	require.NoError(t, err)
	s, err := gossh.ParsePrivateKey(plain)
	require.NoError(t, err)
	return s
}

func csr(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, priv)
	require.NoError(t, err)
	return der
}

func parseCert(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	blk, _ := pem.Decode(certPEM)
	require.NotNil(t, blk)
	c, err := x509.ParseCertificate(blk.Bytes)
	require.NoError(t, err)
	return c
}
