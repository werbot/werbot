package pgconnector

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
)

func probeDial(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}

// probeDB answers the probe's StartupMessage with first and reports on sent
// whether the probe sent anything after it (a password).
func probeDB(t *testing.T, first pgproto3.BackendMessage, sent chan<- bool) *fakeDB {
	t.Helper()
	return newFakeDB(t, nil, func(b *pgproto3.Backend, conn net.Conn, sm *pgproto3.StartupMessage) {
		assert.Equal(t, "app", sm.Parameters["user"])
		b.Send(first)
		_ = b.Flush()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _ := conn.Read(make([]byte, 1))
		sent <- n > 0
	})
}

func TestProbe(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cfg, _ := realPG(t)
	require.NoError(t, Connector{}.Probe(ctx, testResource(t, cfg), probeDial), "SCRAM request means alive")

	for _, tc := range []struct {
		name  string
		first pgproto3.BackendMessage
		err   string
	}{
		{"password request", &pgproto3.AuthenticationMD5Password{Salt: [4]byte{1, 2, 3, 4}}, ""},
		{"cleartext request", &pgproto3.AuthenticationCleartextPassword{}, ""},
		{"auth refused", &pgproto3.ErrorResponse{Severity: "FATAL", Code: "28000", Message: "no pg_hba.conf entry"}, ""},
		{"no database", &pgproto3.ErrorResponse{Severity: "FATAL", Code: "3D000", Message: `database "main" does not exist`}, `3D000: database "main" does not exist`},
	} {
		sent := make(chan bool, 1)
		f := probeDB(t, tc.first, sent)
		cfg := &postgresv1.Config{Host: "127.0.0.1", Port: f.port, Database: "main", Username: "app", Sslmode: "disable"}
		err := Connector{}.Probe(ctx, testResource(t, cfg), probeDial)
		if tc.err == "" {
			require.NoError(t, err, tc.name)
		} else {
			require.EqualError(t, err, tc.err, tc.name)
		}
		assert.False(t, <-sent, "%s: the probe sends no password", tc.name)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().(*net.TCPAddr)
	require.NoError(t, ln.Close())
	port := uint32(addr.Port) //nolint:gosec // a TCP port
	closed := &postgresv1.Config{Host: "127.0.0.1", Port: port, Database: "main", Username: "app", Sslmode: "disable"}
	require.Error(t, Connector{}.Probe(ctx, testResource(t, closed), probeDial), "a closed port is down")
}

func TestProbeTLS(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ca := newCert(t, nil, "")
	f := tlsDB(t, ca, nil)
	cfg := &postgresv1.Config{Host: "localhost", Port: f.port, Database: "main", Username: "app", CaPem: ca.certPEM()}
	require.NoError(t, Connector{}.Probe(ctx, testResource(t, cfg), probeDial))

	cfg.CaPem = newCert(t, nil, "").certPEM()
	require.ErrorContains(t, Connector{}.Probe(ctx, testResource(t, cfg), probeDial), "certificate")

	plain := newFakeDB(t, nil, func(*pgproto3.Backend, net.Conn, *pgproto3.StartupMessage) {})
	cfg = &postgresv1.Config{Host: "localhost", Port: plain.port, Database: "main", Username: "app", Sslmode: "verify-ca", CaPem: ca.certPEM()}
	require.Error(t, Connector{}.Probe(ctx, testResource(t, cfg), probeDial), "a server without TLS is refused unless disable")

	cfg = &postgresv1.Config{Host: "127.0.0.1", Port: f.port, Database: "main", Username: "app", Sslmode: "verify-ca", CaPem: ca.certPEM()}
	require.NoError(t, Connector{}.Probe(ctx, testResource(t, cfg), probeDial), "verify-ca does not check the host")
}

func TestProbeUsesDial(t *testing.T) {
	refused := errors.New("dial refused")
	var got string
	dial := func(_ context.Context, _, address string) (net.Conn, error) { got = address; return nil, refused }
	cfg := &postgresv1.Config{Host: "db.example", Database: "main", Username: "app", Sslmode: "disable"}
	require.ErrorIs(t, Connector{}.Probe(t.Context(), testResource(t, cfg), dial), refused)
	assert.Equal(t, "db.example:5432", got)
}
