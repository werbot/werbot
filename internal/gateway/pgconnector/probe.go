package pgconnector

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgproto3"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// Probe checks that the database of r answers a login: it dials through
// dial, starts TLS as sslmode says (without a client certificate), sends a
// StartupMessage as the resource user and closes at the first answer. A
// password request or an authentication error (class 28) means alive; no
// credential is ever sent.
func (Connector) Probe(ctx context.Context, r gateway.Resource, dial func(ctx context.Context, network, address string) (net.Conn, error)) error {
	cfg, err := resourceConfig(r)
	if err != nil {
		return err
	}
	raw, err := dial(ctx, "tcp", net.JoinHostPort(cfg.GetHost(), strconv.FormatUint(uint64(cfg.GetPort()), 10)))
	if err != nil {
		return err
	}
	defer raw.Close()
	defer context.AfterFunc(ctx, func() { _ = raw.Close() })()
	conn, err := probeTLS(ctx, raw, cfg)
	if err != nil {
		return err
	}
	f := pgproto3.NewFrontend(conn, conn)
	f.Send(&pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersion30,
		Parameters: map[string]string{"user": cfg.GetUsername(), "database": cfg.GetDatabase()}})
	if err := f.Flush(); err != nil {
		return err
	}
	msg, err := f.Receive()
	if err != nil {
		return err
	}
	return probeAnswer(msg)
}

// probeTLS starts TLS on conn unless sslmode is disable; the server must accept it.
func probeTLS(ctx context.Context, conn net.Conn, cfg *postgresv1.Config) (net.Conn, error) {
	tc, err := dbproxy.ClientTLS(cfg.GetSslmode(), cfg.GetCaPem(), cfg.GetHost(), nil)
	if err != nil || tc == nil {
		return conn, err
	}
	req, _ := (&pgproto3.SSLRequest{}).Encode(nil)
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}
	var answer [1]byte
	if _, err := io.ReadFull(conn, answer[:]); err != nil {
		return nil, err
	}
	if answer[0] != 'S' {
		return nil, errors.New("postgres: the server does not accept TLS")
	}
	t := tls.Client(conn, tc)
	return t, t.HandshakeContext(ctx)
}

// probeAnswer is the health of a database by its first answer to a login.
func probeAnswer(msg pgproto3.BackendMessage) error {
	switch m := msg.(type) {
	case *pgproto3.AuthenticationOk, *pgproto3.AuthenticationCleartextPassword, *pgproto3.AuthenticationMD5Password,
		*pgproto3.AuthenticationSASL, *pgproto3.AuthenticationGSS:
		return nil
	case *pgproto3.ErrorResponse:
		if strings.HasPrefix(m.Code, "28") {
			return nil
		}
		return fmt.Errorf("%s: %s", m.Code, m.Message)
	}
	return fmt.Errorf("postgres: unexpected answer %T", msg)
}
