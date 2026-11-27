package pgconnector

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/connector/postgres"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// fixedSettings pin every connection setting the PG* environment of the
// gateway could otherwise supply; quoted empty values clear a key.
// connect_timeout bounds the whole connect to the database.
const fixedSettings = "sslmode=disable sslcertmode=allow sslnegotiation=postgres connect_timeout=10 " +
	"target_session_attrs=any min_protocol_version=3.0 max_protocol_version=3.0 channel_binding=prefer " +
	"require_auth='' passfile=/dev/null sslrootcert='' sslcert='' sslkey=''"

var quoter = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

// quote is v as a libpq keyword value.
func quote(v string) string { return "'" + quoter.Replace(v) + "'" }

// dialConfig is the connection to database db of cfg as user with secret:
// TLS from sslmode, the run-time parameters params, dialing through dial.
// Nothing comes from the PG* environment; an environment that breaks the
// settings (an unreadable PGSERVICE) refuses the connect.
func dialConfig(cfg *postgresv1.Config, secret gateway.Secret, user string, params map[string]string, db string, dial pgconn.DialFunc) (*pgconn.Config, error) {
	var password string
	var clientCert []byte
	switch connector.CredentialKind(secret.Kind) {
	case connector.CredentialPassword:
		password = string(secret.Value)
	case connector.CredentialClientCert:
		clientCert = secret.Value
	}
	tlsCfg, err := dbproxy.ClientTLS(cfg.GetSslmode(), cfg.GetCaPem(), cfg.GetHost(), clientCert)
	if err != nil {
		return nil, err
	}
	port := cmp.Or(uint16(cfg.GetPort()), postgres.DefaultPort) //nolint:gosec // Validate caps the port at 65535
	pc, err := pgconn.ParseConfig(strings.Join([]string{
		"host=" + quote(cfg.GetHost()), "port=" + strconv.Itoa(int(port)),
		"dbname=" + quote(db), "user=" + quote(user), fixedSettings,
	}, " "))
	if err != nil {
		return nil, fmt.Errorf("postgres: connection settings: %w", err)
	}
	pc.Host, pc.Port, pc.Database, pc.User = cfg.GetHost(), port, db, user
	pc.Password, pc.TLSConfig, pc.DialFunc = password, tlsCfg, dial
	pc.Fallbacks, pc.ValidateConnect, pc.AfterConnect, pc.AfterNetConnect, pc.OAuthTokenProvider = nil, nil, nil, nil, nil
	pc.KerberosSrvName, pc.KerberosSpn, pc.SSLCertMode = "", "", "allow"
	pc.RuntimeParams = params
	return pc, nil
}

// runtimeParams are the client's start parameters a person's session gets:
// all but the ones the gateway decides (user, database, replication),
// protocol options and the lexing settings, which the classifier assumes:
// standard_conforming_strings is on.
func runtimeParams(params map[string]string) map[string]string {
	out := make(map[string]string, len(params)+1)
	for k, v := range params {
		switch {
		case k == "user", k == "database", k == "replication", k == "standard_conforming_strings", k == "backslash_quote":
		case !strings.HasPrefix(k, "_pq_."):
			out[k] = v
		}
	}
	out["standard_conforming_strings"] = "on"
	return out
}

// connect logs in to the database with pc and takes over the connection;
// its PgConn stays only as the sender of cancel requests.
func connect(ctx context.Context, pc *pgconn.Config) (*pgconn.HijackedConn, *pgconn.PgConn, error) {
	conn, err := pgconn.ConnectConfig(ctx, pc)
	if err != nil {
		return nil, nil, err
	}
	if err := conn.SyncConn(ctx); err != nil {
		_ = conn.Close(ctx)
		return nil, nil, err
	}
	hc, err := conn.Hijack()
	if err != nil {
		_ = conn.Close(ctx)
		return nil, nil, err
	}
	canceler, err := pgconn.Construct(hc)
	if err != nil {
		_ = hc.Conn.Close()
		return nil, nil, err
	}
	return hc, canceler, nil
}

// connectFailed tells the client that the database login failed: a database
// error keeps its severity and SQLSTATE, anything else is 08006; the text
// names the resource, never its address or credential.
func connectFailed(w io.Writer, s *gateway.Session, err error) error {
	e := &pgproto3.ErrorResponse{Severity: "FATAL", SeverityUnlocalized: "FATAL", Code: "08006",
		Message: "werbot: connection to " + s.Resource.Name + " failed"}
	if pe, ok := errors.AsType[*pgconn.PgError](err); ok {
		e.Severity, e.SeverityUnlocalized, e.Code = pe.Severity, pe.SeverityUnlocalized, pe.Code
		e.Message += ": " + s.Secret.Redact(pe.Message)
	}
	sendFatal(w, e)
	return fmt.Errorf("postgres: connect: %w", err)
}
