package gateway

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func TestTunnelUnaryCalls(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	ip := netip.MustParseAddr("192.0.2.7")

	u, err := e.c.AuthenticateUser(t.Context(), Identity{Method: "ssh-key", Value: "SHA256:known"}, ip)
	require.NoError(t, err)
	require.Equal(t, User{ID: testUserID, Login: "alice", TeamIDs: []uuid.UUID{testTeamID}, Country: "NL"}, u)
	_, err = e.c.AuthenticateUser(t.Context(), Identity{Method: "ssh-key", Value: "SHA256:other"}, ip)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	e.f.with(func(f *fakeTunnel) { require.Equal(t, []string{"192.0.2.7", "192.0.2.7"}, f.unaryIP) })

	id, sec, err := e.c.StartSession(t.Context(), testUserID, uuid.New(), ip, KindStream, "db:5432", "", nil)
	require.NoError(t, err)
	require.Equal(t, testSessionID, id)
	require.Equal(t, Secret{Kind: "password", Value: []byte("pw")}, sec)
	_, _, err = e.c.StartSession(t.Context(), testUserID, uuid.Nil, ip, KindTerminal, "", "", nil)
	require.ErrorContains(t, err, "no credential")

	require.NoError(t, e.c.EndSession(t.Context(), id, "closed", nil))
	require.NoError(t, e.c.EndSession(t.Context(), id, "idle_timeout", &gatewayv1.SessionCounters{BytesUp: 3, DatagramsDown: 1}))
	e.f.with(func(f *fakeTunnel) {
		require.Equal(t, testSessionID.String(), f.ends[0].GetSessionId())
		require.Equal(t, "closed", f.ends[0].GetReason())
		require.Nil(t, f.ends[0].GetCounters())
		require.Equal(t, "idle_timeout", f.ends[1].GetReason())
		require.True(t, proto.Equal(&gatewayv1.SessionCounters{SessionId: testSessionID.String(), BytesUp: 3, DatagramsDown: 1}, f.ends[1].GetCounters()),
			"the counters carry the session id")
	})
}

func TestTunnelUnaryDeadline(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	start := time.Now()
	_, _, err := e.c.StartSession(t.Context(), uuid.Nil, uuid.New(), netip.MustParseAddr("192.0.2.7"), KindTerminal, "", "", nil) // the fake hangs
	require.Less(t, time.Since(start), time.Second)
	require.Equal(t, connect.CodeDeadlineExceeded, connect.CodeOf(err))
}

func TestTunnelTransportConfig(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	roots := x509.NewCertPool()
	tr := newTransport(roots, e.c.certs, "tunnel.example")
	require.True(t, tr.Protocols.HTTP2())
	require.False(t, tr.Protocols.HTTP1())
	require.False(t, tr.Protocols.UnencryptedHTTP2())
	require.Equal(t, 15*time.Second, tr.HTTP2.SendPingTimeout)
	require.Equal(t, 10*time.Second, tr.HTTP2.PingTimeout)
	tc := tr.TLSClientConfig
	require.Equal(t, uint16(tls.VersionTLS13), tc.MinVersion)
	require.Same(t, roots, tc.RootCAs)
	require.Equal(t, "tunnel.example", tc.ServerName)
	require.Equal(t, []string{"h2"}, tc.NextProtos)
	got, err := tc.GetClientCertificate(nil)
	require.NoError(t, err)
	require.Same(t, e.c.certs.cur.Load(), got)
}

func TestTunnelRenewSwapsCertificate(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	e.run(t, nil)
	require.Eventually(t, e.c.Connected, 2*time.Second, time.Millisecond)
	oldCert, _ := os.ReadFile(filepath.Join(e.dir, certFile))
	oldKey, _ := os.ReadFile(filepath.Join(e.dir, keyFile))
	old := e.c.certs.Leaf()

	require.NoError(t, e.c.renew(t.Context()))
	require.Eventually(t, func() bool { return e.f.attempts() == 2 && e.c.Connected() }, 2*time.Second, time.Millisecond)

	e.f.mu.Lock()
	first, second := e.f.peers[0], e.f.peers[1]
	csr, err := x509.ParseCertificateRequest(e.f.csrs[0])
	e.f.mu.Unlock()
	require.NoError(t, err)
	require.Equal(t, old.SerialNumber, first.SerialNumber)
	require.NotEqual(t, first.SerialNumber, second.SerialNumber)
	require.Equal(t, first.Subject.CommonName, second.Subject.CommonName)
	require.Equal(t, e.c.certs.Leaf().SerialNumber, second.SerialNumber)
	require.NotEqual(t, old.PublicKey, csr.PublicKey, "renewal rotates the key")

	newCert, _ := os.ReadFile(filepath.Join(e.dir, certFile))
	newKey, _ := os.ReadFile(filepath.Join(e.dir, keyFile))
	require.NotEqual(t, oldCert, newCert)
	require.NotEqual(t, oldKey, newKey)
	_, err = loadCertStore(e.dir)
	require.NoError(t, err, "the files on disk are a matching pair")
}

func TestTunnelRenewsOnTimer(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Second) // the skewed NotBefore puts renewAt in the past
	old := e.c.certs.Leaf().SerialNumber
	e.run(t, nil)
	require.Eventually(t, func() bool {
		e.f.mu.Lock()
		defer e.f.mu.Unlock()
		n := len(e.f.peers)
		return len(e.f.csrs) == 1 && n > 0 && e.f.peers[n-1].SerialNumber.Cmp(old) != 0
	}, 2*time.Second, time.Millisecond)
	require.Eventually(t, e.c.Connected, 2*time.Second, time.Millisecond)
}

func TestPinHostKeyMismatch(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	rid := uuid.New()
	require.NoError(t, e.c.PinHostKey(t.Context(), rid, "ssh-ed25519", []byte("key")))
	e.f.with(func(f *fakeTunnel) {
		require.Equal(t, rid.String(), f.pins[0].GetResourceId())
		require.Equal(t, "ssh-ed25519", f.pins[0].GetAlg())
		require.Equal(t, []byte("key"), f.pins[0].GetPublicKey())
		f.pinErr = connect.NewError(connect.CodeFailedPrecondition, errors.New("host key mismatch"))
	})
	err := e.c.PinHostKey(t.Context(), rid, "ssh-ed25519", []byte("other"))
	require.ErrorIs(t, err, ErrHostKeyMismatch)

	e.f.with(func(f *fakeTunnel) { f.pinErr = connect.NewError(connect.CodeInternal, errors.New("db down")) })
	err = e.c.PinHostKey(t.Context(), rid, "ssh-ed25519", []byte("other"))
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrHostKeyMismatch)
}

func TestSecretRedactedInLogs(t *testing.T) {
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("start", "secret", Secret{Kind: "password", Value: []byte("hunter2")})
	require.Contains(t, buf.String(), "[redacted]")
	require.NotContains(t, buf.String(), "hunter2")
	require.NotContains(t, buf.String(), "password")
}
