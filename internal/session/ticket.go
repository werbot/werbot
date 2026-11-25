package session

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/session/dbq"
)

// Stream ticket timings.
const (
	StreamTicketTTL        = 30 * time.Second // a ticket is spent within this time or never
	StreamTicketGCAfter    = 5 * time.Minute  // an expired ticket is deleted this long after
	StreamTicketGCInterval = 5 * time.Minute  // how often stream_ticket.gc runs
)

// Window bounds of a web terminal ticket.
const (
	maxTicketCols = 500
	maxTicketRows = 200
)

var (
	errTicketNotFound = fmt.Errorf("stream ticket not found: %w", apperr.ErrNotFound)
	errAgentTicket    = fmt.Errorf("agents may not open browser streams: %w", apperr.ErrPermissionDenied)
)

// StreamTicket is a single-use permit for one browser stream through one
// gateway instance: mode "observe" watches SessionID, mode "web" opens a
// terminal on ResourceID.
type StreamTicket struct {
	// InstanceID is the gateway instance the ticket is addressed to;
	// TargetServerID the server replica holding the browser's WebSocket.
	ID, OrgID, UserID, InstanceID, TargetServerID uuid.UUID
	Mode                                          string
	Login, ResourceName                           string // set by LookupStreamTicket only
	SessionID, ResourceID                         *uuid.UUID
	Cols, Rows                                    int
	ClientIP                                      netip.Addr // the browser's address; zero when unknown
	UserSessionID                                 *uuid.UUID // the browser's cookie session: the MFA binding web:<id>
	ExpiresAt                                     time.Time
}

// CreateObserveTicket issues an observe ticket for the live terminal session
// sessionID, addressed to the session's gateway instance, and audits
// "session.observe" in the same transaction. Owner and admin observe any
// session of the organization, a member only their own; a session that is
// not visible, not live or not a terminal is apperr.ErrNotFound and an agent
// principal is apperr.ErrPermissionDenied.
func (s *Service) CreateObserveTicket(ctx context.Context, p principal.Principal, sessionID, target uuid.UUID) (StreamTicket, error) {
	if p.IsAgent() {
		return StreamTicket{}, errAgentTicket
	}
	arg := dbq.ObservableSessionParams{ID: sessionID, OrgID: p.OrgID}
	if !seesAll(p) {
		arg.UserID = &p.UserID
	}
	var tk StreamTicket
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		instance, err := st.ObservableSession(ctx, arg)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("live session not found: %w", apperr.ErrNotFound)
		}
		if err != nil {
			return err
		}
		tk, err = insertTicket(ctx, st, p, dbq.InsertStreamTicketParams{
			Mode: "observe", SessionID: &sessionID, GatewayInstanceID: instance, TargetServerID: target})
		if err != nil {
			return err
		}
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &p.OrgID, ActorUserID: &p.UserID, ActorKind: "user", Action: "session.observe",
			TargetType: "session", TargetID: sessionID.String(), IP: clientip.From(ctx), Data: map[string]any{"session_id": sessionID},
		})
	})
	return tk, ticketErr("observe ticket", err)
}

// CreateTerminalTicket issues a web terminal ticket for the resource named
// resource with a cols x rows window, addressed to a random online instance
// of the resource's pool that serves its type. An unknown resource is
// apperr.ErrNotFound, one without a pool or an online instance
// apperr.ErrUnavailable, a window outside 1-500 x 1-200
// apperr.ErrInvalidArgument and an agent principal apperr.ErrPermissionDenied.
// Access to the resource is the gateway's decision; the ticket is not audited.
func (s *Service) CreateTerminalTicket(ctx context.Context, p principal.Principal, resource string, cols, rows int, target uuid.UUID) (StreamTicket, error) {
	if p.IsAgent() {
		return StreamTicket{}, errAgentTicket
	}
	if cols < 1 || cols > maxTicketCols || rows < 1 || rows > maxTicketRows {
		return StreamTicket{}, fmt.Errorf("terminal window must be 1-%d x 1-%d: %w", maxTicketCols, maxTicketRows, apperr.ErrInvalidArgument)
	}
	arg := dbq.InsertStreamTicketParams{Mode: "web", Cols: int32(cols), Rows: int32(rows), TargetServerID: target}
	var tk StreamTicket
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		t, err := st.TerminalTarget(ctx, dbq.TerminalTargetParams{OrgID: p.OrgID, Name: resource})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
		}
		if err != nil {
			return err
		}
		if t.InstanceID == uuid.Nil {
			return fmt.Errorf("no online gateway serves the resource: %w", apperr.ErrUnavailable)
		}
		arg.ResourceID, arg.GatewayInstanceID = &t.ResourceID, t.InstanceID
		tk, err = insertTicket(ctx, st, p, arg)
		return err
	})
	return tk, ticketErr("terminal ticket", err)
}

// insertTicket stores arg as a ticket of p from the request's client address
// and cookie session (deleted with it), valid for StreamTicketTTL.
func insertTicket(ctx context.Context, st Store, p principal.Principal, arg dbq.InsertStreamTicketParams) (StreamTicket, error) {
	arg.OrgID, arg.UserID, arg.TtlSecs = p.OrgID, p.UserID, StreamTicketTTL.Seconds()
	if p.SessionID != uuid.Nil {
		arg.UserSessionID = &p.SessionID
	}
	if ip := clientip.From(ctx); ip.IsValid() {
		arg.ClientIp = &ip
	}
	row, err := st.InsertStreamTicket(ctx, arg)
	return ticketOf(row), err
}

// ConsumeStreamTicket spends the ticket id atomically: it is the only spend
// of a ticket. An unknown, expired or already spent ticket is
// apperr.ErrNotFound. It runs without an organization: the caller checks the
// ticket's organization and instance against its peer.
func (s *Service) ConsumeStreamTicket(ctx context.Context, id uuid.UUID) (StreamTicket, error) {
	var row dbq.StreamTicket
	err := dbx.System(ctx, s.pool, dbx.ReasonCluster, func(tx pgx.Tx) (err error) {
		row, err = newStore(tx).ConsumeStreamTicket(ctx, id)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return StreamTicket{}, errTicketNotFound
	}
	if err != nil {
		return StreamTicket{}, fmt.Errorf("session: consume stream ticket: %w", err)
	}
	return ticketOf(row), nil
}

// LookupStreamTicket reads the unexpired ticket id of orgID addressed to
// instanceID with its user's login and resource name, spent or not, and never
// spends it. Any other ticket is apperr.ErrNotFound.
func (s *Service) LookupStreamTicket(ctx context.Context, orgID, instanceID, id uuid.UUID) (StreamTicket, error) {
	var row dbq.LookupStreamTicketRow
	err := dbx.ReadInOrg(s.pool, orgID, uuid.Nil, func(r *dbx.Read) (err error) {
		row, err = newStore(r).LookupStreamTicket(ctx, dbq.LookupStreamTicketParams{ID: id, OrgID: orgID, InstanceID: instanceID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return StreamTicket{}, errTicketNotFound
	}
	if err != nil {
		return StreamTicket{}, fmt.Errorf("session: lookup stream ticket: %w", err)
	}
	tk := ticketOf(row.StreamTicket)
	tk.Login, tk.ResourceName = row.Login, row.ResourceName
	return tk, nil
}

// GCStreamTickets deletes the tickets expired for more than
// StreamTicketGCAfter and returns how many it deleted.
func (s *Service) GCStreamTickets(ctx context.Context) (n int64, err error) {
	err = dbx.System(ctx, s.pool, dbx.ReasonMaintenance, func(tx pgx.Tx) (err error) {
		n, err = newStore(tx).DeleteExpiredStreamTickets(ctx, StreamTicketGCAfter.Seconds())
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("session: gc stream tickets: %w", err)
	}
	return n, nil
}

func ticketOf(t dbq.StreamTicket) StreamTicket {
	tk := StreamTicket{ID: t.ID, OrgID: t.OrgID, UserID: t.UserID, InstanceID: t.GatewayInstanceID, TargetServerID: t.TargetServerID,
		Mode: t.Mode, SessionID: t.SessionID, ResourceID: t.ResourceID, Cols: int(t.Cols), Rows: int(t.Rows), UserSessionID: t.UserSessionID,
		ExpiresAt: t.ExpiresAt}
	if t.ClientIp != nil {
		tk.ClientIP = *t.ClientIp
	}
	return tk
}

// ticketErr keeps the not-found and unavailable errors of a ticket use case
// as they are and wraps any other with the use case op.
func ticketErr(op string, err error) error {
	if err == nil || errors.Is(err, apperr.ErrNotFound) || errors.Is(err, apperr.ErrUnavailable) {
		return err
	}
	return fmt.Errorf("session: %s: %w", op, err)
}
