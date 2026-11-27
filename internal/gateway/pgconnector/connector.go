// Package pgconnector is the gateway connector of the "postgres" resource
// type: the client speaks the PostgreSQL protocol over the stream without a
// password, the gateway logs in to the database with the resource credential.
package pgconnector

import (
	"bufio"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/connector/postgres"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

func init() { gateway.RegisterConnector(Connector{}) }

// ioBuf is the read buffer of the client side of a session.
const ioBuf = 32 << 10

// cancels holds the cancel keys of this gateway's live sessions.
var cancels dbproxy.CancelRegistry

// Connector serves "postgres" sessions.
type Connector struct{}

// Type is "postgres".
func (Connector) Type() string { return postgres.TypeName }

// Kinds is stream only.
func (Connector) Kinds() []gateway.Kind { return []gateway.Kind{gateway.KindStream} }

// DialsTarget marks the connector as dialing through Session.Dial.
func (Connector) DialsTarget() {}

// Serve reads the start PreSession admitted, logs in to the database, hands
// the client its own login answer and relays the session until either side
// ends. A cancel of ctx is not an error; a relay panic re-panics here.
func (Connector) Serve(ctx context.Context, s *gateway.Session) error {
	if s.Stream == nil {
		return errors.New("postgres: remote forwards are not supported")
	}
	defer s.Stream.Close()
	cr := bufio.NewReaderSize(s.Stream, ioBuf)
	msg, cfg, db, err := start(cr, s)
	if err != nil {
		return err
	}
	hc, canceler, err := connect(ctx, s, cfg, msg.Parameters, db)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return connectFailed(s.Stream, s, err)
	}
	defer hc.Conn.Close()
	t0 := time.Now()
	g := dbproxy.NewGuard(s, dbproxy.GuardConfig{Type: postgres.TypeName, Metrics: s.Metrics})
	defer g.Close()
	key, release := cancels.Register(dbproxy.CancelTarget{
		ResourceID: s.Resource.ID,
		Cancel:     canceler.CancelRequest,
		OnCancel:   func() { g.Log(dbproxy.Entry{T: time.Since(t0), Kind: "cancel", DB: db}) },
	})
	defer release()
	if _, err := s.Stream.Write(handshake(msg, hc.ParameterStatuses, key, hc.TxStatus)); err != nil {
		return nil // the client is gone
	}
	c, d := newRelay(ctx, cr, hc.Conn, s.Stream, g, sessionLog{t0: t0, db: db, params: cfg.GetRecordParams(), secret: s.Secret})
	return relay(ctx, s, hc, c, d)
}

// start reads the client's StartupMessage and checks it again against the
// resource; a refusal tells the client. It returns the message, the resource
// config and the database.
func start(cr *bufio.Reader, s *gateway.Session) (*pgproto3.StartupMessage, *postgresv1.Config, string, error) {
	cfg, err := resourceConfig(s.Resource)
	if err != nil {
		return nil, nil, "", err
	}
	m, err := readStartup(cr)
	if err != nil {
		return nil, nil, "", err
	}
	db, refusal := admit(cfg, m.Parameters)
	if refusal != "" {
		return nil, nil, "", refuse(s.Stream, refusal)
	}
	return m, cfg, db, nil
}
