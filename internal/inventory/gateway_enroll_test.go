package inventory_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

func enrollReq(tok string, csrDER []byte, host string) inventory.EnrollRequest {
	return inventory.EnrollRequest{JoinToken: tok, CSRDER: csrDER, Hostname: host, Version: "v0.1.0",
		Connectors: []string{"ssh"}, IP: netip.MustParseAddr("192.0.2.10")}
}

func (e gwEnv) enroll(t *testing.T, tok, host string) inventory.Enrollment {
	t.Helper()
	en, err := e.gw.Enroll(context.Background(), enrollReq(tok, csr(t), host))
	require.NoError(t, err)
	return en
}

func (e gwEnv) uses(t *testing.T, tok string) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT uses FROM gateway_join_token WHERE token_hash = sha256($1::bytea)", []byte(secret(tok))).Scan(&n))
	return n
}

func TestEnrollCreatesInstance(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	tokID, tok, _, err := e.gw.CreateJoinToken(e.ctx, g.ID, 0, 0)
	require.NoError(t, err)

	en := e.enroll(t, tok, "gw-a")
	assert.Equal(t, g.ID, en.GatewayID)
	assert.Equal(t, uuid.Version(7), en.InstanceID.Version())
	assert.Equal(t, e.ca.RootPEM(), en.CAPEM)
	cert := parseCert(t, en.CertPEM)
	assert.Equal(t, tlsca.GatewayCN(g.ID, en.InstanceID), cert.Subject.CommonName)
	assert.WithinDuration(t, time.Now().Add(inventory.CertTTL), cert.NotAfter, time.Minute)

	var serial, host, ip string
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT cert_serial, hostname, host(client_ip) FROM gateway_instance WHERE id = $1", en.InstanceID).Scan(&serial, &host, &ip))
	assert.Equal(t, cert.SerialNumber.Text(16), serial)
	assert.Equal(t, "gw-a", host)
	assert.Equal(t, "192.0.2.10", ip)

	key, err := gossh.ParsePrivateKey(en.HostKeyPEM)
	require.NoError(t, err)
	assert.Equal(t, gossh.KeyAlgoED25519, key.PublicKey().Type())
	assert.Equal(t, e.poolKey(t, g.ID).PublicKey().Marshal(), key.PublicKey().Marshal())

	en2 := e.enroll(t, tok, "gw-b")
	assert.NotEqual(t, en.InstanceID, en2.InstanceID)
	key2, err := gossh.ParsePrivateKey(en2.HostKeyPEM)
	require.NoError(t, err)
	assert.Equal(t, key.PublicKey().Marshal(), key2.PublicKey().Marshal(), "one host key per pool")
	assert.Equal(t, 2, e.uses(t, tok))

	var kind string
	var orgID uuid.UUID
	var actor *uuid.UUID
	var raw []byte
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT actor_kind, actor_user_id, org_id, data FROM audit_event WHERE action = 'gateway.instance.enroll' AND target_id = $1",
		en.InstanceID.String()).Scan(&kind, &actor, &orgID, &raw))
	assert.Equal(t, "gateway", kind)
	assert.Nil(t, actor)
	assert.Equal(t, e.org, orgID)
	var data map[string]any
	require.NoError(t, json.Unmarshal(raw, &data))
	assert.Equal(t, map[string]any{"gateway_id": g.ID.String(), "instance_id": en.InstanceID.String(),
		"token_id": tokID.String(), "hostname": "gw-a", "client_ip": "192.0.2.10"}, data)
}

func TestJoinTokenMaxUses(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	tok := e.token(t, g.ID, 2)
	e.enroll(t, tok, "a")
	e.enroll(t, tok, "b")
	_, err := e.gw.Enroll(context.Background(), enrollReq(tok, csr(t), "c"))
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.Equal(t, 2, e.uses(t, tok))

	g3 := e.newPool(t, "race")
	tok3 := e.token(t, g3.ID, 3)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		der := csr(t)
		wg.Go(func() {
			_, err := e.gw.Enroll(context.Background(), enrollReq(tok3, der, "r"))
			if err == nil {
				ok.Add(1)
			} else {
				assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
			}
		})
	}
	wg.Wait()
	assert.EqualValues(t, 3, ok.Load())
	assert.Equal(t, 3, e.uses(t, tok3))
	assert.Equal(t, 3, e.count(t, "SELECT count(*) FROM gateway_instance WHERE gateway_id = $1", g3.ID))
}

func TestEnrollRejects(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	enroll := func(tok string, der []byte) error {
		_, err := e.gw.Enroll(context.Background(), enrollReq(tok, der, "h"))
		return err
	}

	expiredID, expired, _, err := e.gw.CreateJoinToken(e.ctx, g.ID, 0, 0)
	require.NoError(t, err)
	_, err = e.pool.Exec(context.Background(), "UPDATE gateway_join_token SET expires_at = now() - interval '1 second' WHERE id = $1", expiredID)
	require.NoError(t, err)
	revokedID, revoked, _, err := e.gw.CreateJoinToken(e.ctx, g.ID, 0, 0)
	require.NoError(t, err)
	require.NoError(t, e.gw.RevokeJoinToken(e.ctx, revokedID))
	assert.Equal(t, 1, e.count(t,
		"SELECT count(*) FROM audit_event WHERE action = 'gateway.join_token.revoke' AND target_id = $1", revokedID.String()))
	exhausted := e.token(t, g.ID, 1)
	e.enroll(t, exhausted, "first")
	unknown := "wjt_" + base64.RawURLEncoding.EncodeToString(make([]byte, 32)) + "." + e.ca.Fingerprint()

	var texts []string
	for name, tok := range map[string]string{"expired": expired, "revoked": revoked, "exhausted": exhausted,
		"unknown": unknown, "malformed": "garbage"} {
		err := enroll(tok, csr(t))
		require.ErrorIs(t, err, apperr.ErrPermissionDenied, name)
		texts = append(texts, err.Error())
	}
	for _, s := range texts {
		assert.Equal(t, texts[0], s, "one message for every bad token")
	}

	good := e.token(t, g.ID, 0)
	foreign := strings.TrimSuffix(good, e.ca.Fingerprint()) + strings.Repeat("0", 64)
	assert.ErrorIs(t, enroll(foreign, csr(t)), apperr.ErrInvalidArgument)
	assert.Equal(t, 0, e.uses(t, good))

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecCSR, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, priv)
	require.NoError(t, err)
	before := e.count(t, "SELECT count(*) FROM gateway_instance")
	assert.ErrorIs(t, enroll(good, ecCSR), apperr.ErrInvalidArgument)
	assert.Equal(t, 0, e.uses(t, good), "the spend rolls back")
	assert.Equal(t, before, e.count(t, "SELECT count(*) FROM gateway_instance"))
}

func TestAuthenticateChecksSerial(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	en := e.enroll(t, e.token(t, g.ID, 0), "gw-a")
	serial := parseCert(t, en.CertPEM).SerialNumber.Text(16)
	bg := context.Background()

	inst, err := e.gw.Authenticate(bg, g.ID, en.InstanceID, serial)
	require.NoError(t, err)
	assert.Equal(t, en.InstanceID, inst.ID)
	assert.Equal(t, g.ID, inst.GatewayID)
	assert.Equal(t, e.org, inst.OrgID)

	other := e.newPool(t, "other")
	enOther := e.enroll(t, e.token(t, other.ID, 0), "gw-o")
	_, err = e.gw.Authenticate(bg, g.ID, enOther.InstanceID, parseCert(t, enOther.CertPEM).SerialNumber.Text(16))
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated, "instance of another pool")
	_, err = e.gw.Authenticate(bg, g.ID, en.InstanceID, "00")
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)

	renewed, err := e.gw.Renew(bg, en.InstanceID, csr(t))
	require.NoError(t, err)
	newCert := parseCert(t, renewed)
	assert.Equal(t, tlsca.GatewayCN(g.ID, en.InstanceID), newCert.Subject.CommonName)
	_, err = e.gw.Authenticate(bg, g.ID, en.InstanceID, serial)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated, "old serial")
	_, err = e.gw.Authenticate(bg, g.ID, en.InstanceID, newCert.SerialNumber.Text(16))
	require.NoError(t, err)
	assert.Equal(t, 1, e.count(t, "SELECT count(*) FROM audit_event WHERE target_id = $1", en.InstanceID.String()),
		"Renew writes no audit")

	_, err = e.gw.Renew(bg, uuid.New(), csr(t))
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
}

func TestHeartbeatMarksOnline(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	en := e.enroll(t, e.token(t, g.ID, 0), "gw-a")
	instance := func() inventory.Instance {
		list, err := e.gw.List(e.ctx)
		require.NoError(t, err)
		require.Len(t, list[0].Instances, 1)
		return list[0].Instances[0]
	}
	assert.False(t, instance().Online)

	require.NoError(t, e.gw.Heartbeat(context.Background(), en.InstanceID, "v0.2.0", []string{"ssh", "echo"}))
	in := instance()
	assert.True(t, in.Online)
	assert.Equal(t, "v0.2.0", in.Version)
	assert.Equal(t, []string{"ssh", "echo"}, in.Connectors)
	assert.Equal(t, 1, e.count(t, "SELECT count(*) FROM audit_event WHERE target_id = $1", en.InstanceID.String()),
		"Heartbeat writes no audit")
	assert.ErrorIs(t, e.gw.Heartbeat(context.Background(), uuid.New(), "v", nil), apperr.ErrUnauthenticated)
}
