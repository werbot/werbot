package pgconnector

import (
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
)

// The databases stand (deploy/e2e/databases.yaml): its test-only superuser
// and the CA of its server certificate.
const (
	standUser     = "postgres"
	standPassword = "password"
	standCA       = "../../../deploy/e2e/databases/tls/ca.pem"
)

// TestVerifyFullRealPostgres connects through the connector to the stand's
// PostgreSQL over TLS with sslmode verify-full: the stand's CA passes,
// another CA refuses the session with 08006. WERBOT_TEST_POSTGRES is the
// stand's host:port (make e2e-databases).
func TestVerifyFullRealPostgres(t *testing.T) {
	addr := os.Getenv("WERBOT_TEST_POSTGRES")
	if addr == "" {
		t.Skip("WERBOT_TEST_POSTGRES is not set (make e2e-databases)")
	}
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	n, err := strconv.ParseUint(port, 10, 16)
	require.NoError(t, err)
	ca, err := os.ReadFile(standCA)
	require.NoError(t, err)
	cfg := &postgresv1.Config{Host: host, Port: uint32(n), Database: "postgres", Username: standUser, CaPem: string(ca)}

	p := serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(standPassword))
	conn, err := pgconn.Connect(t.Context(), p.connectURL("postgres"))
	require.NoError(t, err, "verify-full with the stand's CA")
	res, err := conn.Exec(t.Context(), "select ssl from pg_stat_ssl where pid = pg_backend_pid()").ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "t", string(res[0].Rows[0][0]), "the database session is TLS")
	require.NoError(t, conn.Close(t.Context()))

	cfg.CaPem = newCert(t, nil, "").certPEM()
	p = serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(standPassword))
	_, err = pgconn.Connect(t.Context(), p.connectURL("postgres"))
	assert.Equal(t, "08006", pgErr(t, err).Code)
	assert.ErrorContains(t, p.serveErr(t), "certificate")
}
