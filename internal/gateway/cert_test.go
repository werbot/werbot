package gateway

import (
	"crypto/rand"
	"net/netip"
	"strings"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/sshca"
)

func authorizedLine(k gossh.PublicKey) string {
	return strings.TrimSuffix(string(gossh.MarshalAuthorizedKey(k)), "\n")
}

// certEnv is a clocked SSH front whose snapshot trusts ca.
type certEnv struct {
	*sshEnv
	clk          *testClock
	ca           gossh.Signer
	org, session uuid.UUID
}

func newCertEnv(t *testing.T) *certEnv {
	t.Helper()
	e, clk := newClockedSSHEnv(t)
	c := &certEnv{sshEnv: e, clk: clk, ca: newKey(t), org: uuid.New(), session: uuid.New()}
	snap := testSnapshot(t, e.m, sshPolicies, nil, res{"rdb", "db", "fake"})
	snap.UserCaKeys = []string{authorizedLine(c.ca.PublicKey())}
	pol, err := CompilePolicy(snap)
	require.NoError(t, err)
	e.f.with(func(f *fakeControl) { f.policy = pol })
	return c
}

// cert signs a certificate for alice's werbot identity with ca at the env's
// clock; edit changes it before it is signed again.
func (c *certEnv) cert(t *testing.T, ca gossh.Signer, edit func(*gossh.Certificate)) (gossh.Signer, *gossh.Certificate) {
	t.Helper()
	key := newKey(t)
	cert, err := sshca.Sign(ca, key.PublicKey(), sshca.Identity{OrgID: c.org, UserID: c.m["u1"], CLISessionID: c.session, Login: "alice"},
		c.clk.now(), sshca.UserCertTTL)
	require.NoError(t, err)
	if edit != nil {
		edit(cert)
		require.NoError(t, cert.SignCert(rand.Reader, ca))
	}
	s, err := gossh.NewCertSigner(cert, key)
	require.NoError(t, err)
	return s, cert
}

// accept makes the fake control plane know the certificate as alice.
func (c *certEnv) accept(cert *gossh.Certificate) {
	c.f.with(func(f *fakeControl) { f.byKey[gossh.FingerprintSHA256(cert)] = User{ID: c.m["u1"], Login: "alice"} })
}

func TestCertLogin(t *testing.T) {
	e := newCertEnv(t)
	s, cert := e.cert(t, e.ca, nil)
	e.accept(cert)
	c, err := e.dial("alice+db", s)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client
	out, code := run(t, c, false, "uptime")
	require.Equal(t, 0, code)
	require.Contains(t, out, "fake:db user=alice")

	e.clk.add(handshakeReuse + time.Nanosecond) // the next channel authenticates again, with the certificate
	_, code = run(t, c, false, "uptime")
	require.Equal(t, 0, code)
	want := Identity{Method: "ssh-cert", Value: gossh.FingerprintSHA256(cert),
		Cert: &sshca.Identity{OrgID: e.org, UserID: e.m["u1"], CLISessionID: e.session, Login: "alice"}}
	e.f.with(func(f *fakeControl) {
		require.Equal(t, []Identity{want, want}, f.idents, "the handshake, then the later channel")
	})
}

func TestCertRejected(t *testing.T) {
	e := newCertEnv(t)
	other := newKey(t)
	var refused []string
	for name, c := range map[string]struct {
		ca   gossh.Signer
		edit func(*gossh.Certificate)
	}{
		"CA of another organization": {other, nil},
		"principal of another login": {e.ca, func(c *gossh.Certificate) { c.ValidPrincipals = []string{"bob"} }},
		"no principal":               {e.ca, func(c *gossh.Certificate) { c.ValidPrincipals = nil }},
		"two principals":             {e.ca, func(c *gossh.Certificate) { c.ValidPrincipals = []string{"alice", "bob"} }},
		"expired": {e.ca, func(c *gossh.Certificate) {
			c.ValidBefore = uint64(e.clk.now().Add(-time.Second).Unix()) //nolint:gosec // test time
		}},
		"not yet valid": {e.ca, func(c *gossh.Certificate) {
			c.ValidAfter = uint64(e.clk.now().Add(2 * time.Minute).Unix()) //nolint:gosec // test time
		}},
		"host certificate":      {e.ca, func(c *gossh.Certificate) { c.CertType = gossh.HostCert }},
		"no session extension":  {e.ca, func(c *gossh.Certificate) { delete(c.Extensions, sshca.ExtSession) }},
		"force-command":         {e.ca, func(c *gossh.Certificate) { c.CriticalOptions = map[string]string{"force-command": "id"} }},
		"source-address option": {e.ca, func(c *gossh.Certificate) { c.CriticalOptions = map[string]string{"source-address": "0.0.0.0/0"} }},
	} {
		s, cert := e.cert(t, c.ca, c.edit)
		e.accept(cert) // the server would know it: only the gateway's check refuses
		_, err := e.dial("alice+db", s)
		require.ErrorContains(t, err, "unable to authenticate", name)
		refused = append(refused, gossh.FingerprintSHA256(cert))
	}
	got := e.denialsAfter(t, len(refused))
	var fps []string
	for _, d := range got {
		require.Equal(t, "auth_failed", d.GetReason())
		fps = append(fps, d.GetKeyFingerprint())
	}
	require.ElementsMatch(t, refused, fps, "one auth_failed per refused certificate")
	require.Zero(t, e.authCount(), "a refused certificate never reaches the control plane")
}

func TestCertSessionRevoked(t *testing.T) {
	e := newCertEnv(t)
	s, _ := e.cert(t, e.ca, nil) // the server answers NotFound: the CLI session is revoked
	_, err := e.dial("alice+db", s)
	require.ErrorContains(t, err, "unable to authenticate")
	require.Equal(t, "auth_failed", e.denialsAfter(t, 1)[0].GetReason())

	c, err := e.dial("alice+db", e.keys["alice"])
	require.NoError(t, err, "the member's key still works")
	_ = c.Close()
}

func TestCertNoSnapshotFailsClosed(t *testing.T) {
	e := newCertEnv(t)
	s, cert := e.cert(t, e.ca, nil)
	e.accept(cert)
	e.f.with(func(f *fakeControl) { f.policy = nil })
	c, err := e.dial("alice+db", s)
	require.NoError(t, err, "the handshake succeeds without a snapshot")
	defer c.Close() //nolint:errcheck // test client
	out, code := run(t, c, false, "uptime")
	require.Equal(t, "Gateway is not ready, try again later\r\n", out)
	require.Equal(t, 1, code)
	require.Zero(t, e.startCalls())
}

func TestCompilePolicyUserCAKeys(t *testing.T) {
	m := newIDs("r1")
	ca, other := newKey(t), newKey(t)
	snap := testSnapshot(t, m, testPolicies, nil)

	p, err := CompilePolicy(snap)
	require.NoError(t, err)
	require.False(t, p.IsUserCA(ca.PublicKey()), "no keys: no CA")

	snap.UserCaKeys = []string{authorizedLine(ca.PublicKey())}
	p, err = CompilePolicy(snap)
	require.NoError(t, err)
	require.True(t, p.IsUserCA(ca.PublicKey()))
	require.False(t, p.IsUserCA(other.PublicKey()))

	for _, bad := range []string{"garbage", "ssh-ed25519 AAAA", ""} {
		snap.UserCaKeys = []string{bad}
		_, err = CompilePolicy(snap)
		require.Error(t, err, bad)
	}
}

func TestAuthRequest(t *testing.T) {
	ip := netip.MustParseAddr("192.0.2.7")
	user, session := uuid.New(), uuid.New()
	r := authRequest(Identity{Method: "ssh-key", Value: "SHA256:key"}, ip)
	require.Equal(t, "SHA256:key", r.GetFingerprintSha256())
	require.Equal(t, "192.0.2.7", r.GetClientIp())

	require.NoError(t, protovalidate.Validate(r))

	r = authRequest(Identity{Method: "ssh-cert", Value: "SHA256:cert",
		Cert: &sshca.Identity{OrgID: uuid.New(), UserID: user, CLISessionID: session, Login: "alice"}}, ip)
	require.NoError(t, protovalidate.Validate(r))
	require.True(t, proto.Equal(&gatewayv1.CertIdentity{UserId: user.String(), CliSessionId: session.String(), Login: "alice",
		CertFingerprint: "SHA256:cert"}, r.GetCert()), "%v", r.GetCert())
}
