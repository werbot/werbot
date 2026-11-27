package pgconnector

import (
	"context"
	"crypto/md5" //nolint:gosec // the md5 password exchange of the PostgreSQL protocol
	"crypto/tls"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway"
)

func clientCertSecret(c *testCert) gateway.Secret {
	return gateway.Secret{Kind: string(connector.CredentialClientCert), Value: c.pem}
}

func TestDialConfigIgnoresPGEnv(t *testing.T) {
	dir := t.TempDir()
	passfile := filepath.Join(dir, "pgpass")
	require.NoError(t, os.WriteFile(passfile, []byte("*:*:*:*:from-passfile\n"), 0o600))
	for k, v := range map[string]string{
		"PGHOST": "evil.example", "PGPORT": "6543", "PGUSER": "evil", "PGDATABASE": "evil", "PGPASSWORD": "from-env",
		"PGPASSFILE": passfile, "PGSSLMODE": "disable", "PGSSLNEGOTIATION": "direct", "PGOPTIONS": "-c role=evil",
		"PGAPPNAME": "evil-app", "PGTZ": "Mars/Base", "PGTARGETSESSIONATTRS": "read-write", "PGMAXPROTOCOLVERSION": "3.2",
		"PGMINPROTOCOLVERSION": "3.2", "PGCONNECT_TIMEOUT": "x", "PGSSLCERTMODE": "disable", "PGCHANNELBINDING": "require",
		"PGREQUIREAUTH": "md5", "PGSSLROOTCERT": filepath.Join(dir, "missing-root.crt"),
		"PGSSLCERT": filepath.Join(dir, "missing.crt"), "PGSSLKEY": filepath.Join(dir, "missing.key"),
	} {
		t.Setenv(k, v)
	}
	ca := newCert(t, nil, "")
	client := newCert(t, ca, "werbot-client")
	cfg := &postgresv1.Config{Host: "db.example", Port: 5433, Database: "main", Username: "app", Sslmode: "verify-full", CaPem: ca.certPEM()}
	params := map[string]string{"user": "nobody", "database": "main", "replication": "off", "_pq_.x": "1", "application_name": "psql", "options": "-c search_path=app"}
	dial := func(context.Context, string, string) (net.Conn, error) { return nil, net.ErrClosed }

	for _, secret := range []gateway.Secret{passwordSecret(marker), clientCertSecret(client)} {
		pc, err := dialConfig(cfg, secret, params, "app_1", dial)
		require.NoError(t, err, secret.Kind)
		assert.Equal(t, "db.example", pc.Host)
		assert.Equal(t, uint16(5433), pc.Port)
		assert.Equal(t, "app_1", pc.Database)
		assert.Equal(t, "app", pc.User)
		assert.Equal(t, map[string]string{"application_name": "psql", "options": "-c search_path=app"}, pc.RuntimeParams)
		assert.Equal(t, 10*time.Second, pc.ConnectTimeout)
		assert.Equal(t, "postgres", pc.SSLNegotiation)
		assert.Equal(t, "allow", pc.SSLCertMode)
		assert.Equal(t, "3.0", pc.MinProtocolVersion)
		assert.Equal(t, "3.0", pc.MaxProtocolVersion)
		assert.Equal(t, "prefer", pc.ChannelBinding)
		assert.Empty(t, pc.RequireAuth)
		assert.Empty(t, pc.Fallbacks)
		assert.Nil(t, pc.ValidateConnect)
		assert.Nil(t, pc.AfterConnect)
		assert.Nil(t, pc.AfterNetConnect)
		assert.Nil(t, pc.OAuthTokenProvider)
		assert.Empty(t, pc.KerberosSrvName)
		assert.Empty(t, pc.KerberosSpn)
		require.NotNil(t, pc.TLSConfig)
		assert.Equal(t, "db.example", pc.TLSConfig.ServerName)
		assert.False(t, pc.TLSConfig.InsecureSkipVerify)
		_, err = pc.DialFunc(context.Background(), "tcp", "x")
		assert.ErrorIs(t, err, net.ErrClosed, "DialFunc is the session's dialer")
		if secret.Kind == string(connector.CredentialPassword) {
			if pc.Password != marker {
				t.Fatal("the password is not the resource credential")
			}
			assert.Empty(t, pc.TLSConfig.Certificates)
		} else {
			assert.Empty(t, pc.Password, "no password from PGPASSWORD or PGPASSFILE")
			assert.Len(t, pc.TLSConfig.Certificates, 1, "PGSSLCERTMODE does not drop the client certificate")
		}
	}
}

func TestDialConfigQuotesValues(t *testing.T) {
	cfg := &postgresv1.Config{Host: "db.example", Database: "main", Username: "app", Sslmode: "disable"}
	pc, err := dialConfig(cfg, passwordSecret("p"), nil, `a b' sslmode=require \x`, nil)
	require.NoError(t, err)
	assert.Equal(t, `a b' sslmode=require \x`, pc.Database)
	assert.Nil(t, pc.TLSConfig)
	assert.Equal(t, uint16(5432), pc.Port)
}

func TestDialConfigBadPGServiceFailsClosed(t *testing.T) {
	f := newFakeDB(t, nil, func(b *pgproto3.Backend, _ net.Conn, _ *pgproto3.StartupMessage) { acceptSession(b) })
	cfg := &postgresv1.Config{Host: "127.0.0.1", Port: f.port, Database: "main", Username: "app", Sslmode: "disable"}
	p := serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(marker))
	client, err := pgconn.ParseConfig(p.connectURL("main")) // before the environment breaks
	require.NoError(t, err)
	t.Setenv("PGSERVICE", "missing-service")
	t.Setenv("PGSERVICEFILE", filepath.Join(t.TempDir(), "missing.conf"))
	_, err = pgconn.ConnectConfig(t.Context(), client)
	require.Error(t, err)
	e := pgErr(t, err)
	assert.Equal(t, "08006", e.Code)
	assert.Equal(t, "werbot: connection to pg-test failed", e.Message)
	serr := p.serveErr(t)
	require.Error(t, serr)
	if strings.Contains(serr.Error(), marker) || strings.Contains(err.Error(), marker) {
		t.Fatal("the credential reached an error")
	}
	assert.Zero(t, p.dials.Load(), "nothing is dialed")
}

func TestServeDialsThroughSession(t *testing.T) {
	f := newFakeDB(t, nil, func(b *pgproto3.Backend, _ net.Conn, _ *pgproto3.StartupMessage) { acceptSession(b) })
	cfg := &postgresv1.Config{Host: "127.0.0.1", Port: f.port, Database: "main", Username: "app", Sslmode: "disable"}
	res := testResource(t, cfg)

	p := serveProxy(t.Context(), t, res, passwordSecret(marker))
	conn, err := pgconn.Connect(t.Context(), p.connectURL("main"))
	require.NoError(t, err)
	assert.Equal(t, int32(1), p.dials.Load())
	require.NoError(t, conn.Close(t.Context()))
	require.NoError(t, p.serveErr(t))

	refused := serveProxy(t.Context(), t, res, passwordSecret(marker), func(p *proxy) { p.dialErr = gateway.ErrTargetNotAllowed })
	_, err = pgconn.Connect(t.Context(), refused.connectURL("main"))
	e := pgErr(t, err)
	assert.Equal(t, "08006", e.Code)
	assert.Equal(t, "werbot: connection to pg-test failed", e.Message)
	addr := "127.0.0.1:" + strconv.Itoa(int(f.port))
	assert.NotContains(t, e.Message+e.Detail+e.Hint, addr, "the client never sees the address")
	require.ErrorIs(t, refused.serveErr(t), gateway.ErrTargetNotAllowed)
}

func TestDBAuthCleartextMD5(t *testing.T) {
	got := make(chan string, 1)
	cleartext := newFakeDB(t, nil, func(b *pgproto3.Backend, _ net.Conn, _ *pgproto3.StartupMessage) {
		b.Send(&pgproto3.AuthenticationCleartextPassword{})
		_ = b.Flush()
		_ = b.SetAuthType(pgproto3.AuthTypeCleartextPassword)
		if m, err := b.Receive(); err == nil {
			got <- m.(*pgproto3.PasswordMessage).Password
		}
		acceptSession(b)
	})
	salt := [4]byte{9, 8, 7, 6}
	md5db := newFakeDB(t, nil, func(b *pgproto3.Backend, _ net.Conn, _ *pgproto3.StartupMessage) {
		b.Send(&pgproto3.AuthenticationMD5Password{Salt: salt})
		_ = b.Flush()
		_ = b.SetAuthType(pgproto3.AuthTypeMD5Password)
		if m, err := b.Receive(); err == nil {
			got <- m.(*pgproto3.PasswordMessage).Password
		}
		acceptSession(b)
	})
	inner := md5.Sum([]byte(marker + "app"))                                   //nolint:gosec // the protocol's md5 exchange
	outer := md5.Sum(append([]byte(hex.EncodeToString(inner[:])), salt[:]...)) //nolint:gosec // the protocol's md5 exchange
	for want, f := range map[string]*fakeDB{marker: cleartext, "md5" + hex.EncodeToString(outer[:]): md5db} {
		cfg := &postgresv1.Config{Host: "127.0.0.1", Port: f.port, Database: "main", Username: "app", Sslmode: "disable"}
		p := serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(marker))
		conn, err := pgconn.Connect(t.Context(), p.connectURL("main"))
		require.NoError(t, err)
		if <-got != want {
			t.Fatal("the database got a wrong password answer")
		}
		require.NoError(t, conn.Close(t.Context()))
	}
}

// tlsDB is a fake database presenting a certificate of ca for localhost; it
// reports the common name of a client certificate on cn.
func tlsDB(t *testing.T, ca *testCert, cn chan<- string) *fakeDB {
	t.Helper()
	srv := newCert(t, ca, "localhost")
	pair, err := tls.X509KeyPair(srv.pem, srv.pem)
	require.NoError(t, err)
	tc := &tls.Config{Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequestClientCert, MinVersion: tls.VersionTLS12}
	return newFakeDB(t, tc, func(b *pgproto3.Backend, conn net.Conn, _ *pgproto3.StartupMessage) {
		if st := conn.(*tls.Conn).ConnectionState(); len(st.PeerCertificates) > 0 && cn != nil {
			cn <- st.PeerCertificates[0].Subject.CommonName
		}
		acceptSession(b)
	})
}

func TestVerifyFullWrongCA(t *testing.T) {
	f := tlsDB(t, newCert(t, nil, ""), nil)
	cfg := &postgresv1.Config{Host: "localhost", Port: f.port, Database: "main", Username: "app", CaPem: newCert(t, nil, "").certPEM()}
	p := serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(marker))
	_, err := pgconn.Connect(t.Context(), p.connectURL("main"))
	assert.Equal(t, "08006", pgErr(t, err).Code)
	assert.Contains(t, p.serveErr(t).Error(), "certificate")
}

func TestVerifyFullAndClientCert(t *testing.T) {
	ca := newCert(t, nil, "")
	cn := make(chan string, 1)
	f := tlsDB(t, ca, cn)
	cfg := &postgresv1.Config{Host: "localhost", Port: f.port, Database: "main", Username: "app", CaPem: ca.certPEM()}

	p := serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(marker))
	conn, err := pgconn.Connect(t.Context(), p.connectURL("main"))
	require.NoError(t, err, "verify-full with the right CA and host")
	require.NoError(t, conn.Close(t.Context()))

	p = serveProxy(t.Context(), t, testResource(t, cfg), clientCertSecret(newCert(t, ca, "werbot-client")))
	conn, err = pgconn.Connect(t.Context(), p.connectURL("main"))
	require.NoError(t, err)
	assert.Equal(t, "werbot-client", <-cn)
	require.NoError(t, conn.Close(t.Context()))

	cfg.Host = "127.0.0.1" // not in the server certificate
	p = serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(marker))
	_, err = pgconn.Connect(t.Context(), p.connectURL("main"))
	assert.Equal(t, "08006", pgErr(t, err).Code, "verify-full checks the host")
}

func TestRequireSkipsVerify(t *testing.T) {
	f := tlsDB(t, newCert(t, nil, ""), nil)
	cfg := &postgresv1.Config{Host: "localhost", Port: f.port, Database: "main", Username: "app", Sslmode: "require"}
	p := serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(marker))
	conn, err := pgconn.Connect(t.Context(), p.connectURL("main"))
	require.NoError(t, err)
	require.NoError(t, conn.Close(t.Context()))
}

func TestClientCertNeedsTLS(t *testing.T) {
	cert := clientCertSecret(newCert(t, newCert(t, nil, ""), "werbot-client"))
	cfg := &postgresv1.Config{Host: "127.0.0.1", Port: 1, Database: "main", Username: "app", Sslmode: "disable"}
	_, err := dialConfig(cfg, cert, nil, "main", nil)
	require.EqualError(t, err, "client certificate requires TLS")

	p := serveProxy(t.Context(), t, testResource(t, cfg), cert)
	_, err = pgconn.Connect(t.Context(), p.connectURL("main"))
	assert.Equal(t, "08006", pgErr(t, err).Code)
	assert.ErrorContains(t, p.serveErr(t), "client certificate requires TLS")
	assert.Zero(t, p.dials.Load())
}
