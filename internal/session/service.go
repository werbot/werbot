package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/session/dbq"
)

// Service records sessions and their end.
type Service struct{ pool *pgxpool.Pool }

// New returns a Service over pool.
func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Start records a new open session and audits "session.start", marked
// break_glass under an active break-glass grant of the user. The gateway
// names the user, which the server does not trust: a user who is not a member
// of the organization is apperr.ErrNotFound and gets no session.
func (s *Service) Start(ctx context.Context, p Start) (uuid.UUID, error) {
	if p.Kind == "" {
		p.Kind = "terminal"
	}
	args := []any{p.OrgID, p.UserID, p.ResourceID, p.GatewayID, p.InstanceID, p.ClientIP, p.Kind, p.Target, p.Command, p.Classes, p.Paths}
	var id uuid.UUID
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		data := map[string]any{"kind": p.Kind, "target": p.Target}
		var breakGlass bool
		if err := tx.QueryRow(ctx, activeBreakGlass, p.OrgID, p.UserID, p.ResourceID).Scan(&breakGlass); err != nil {
			return err
		}
		if breakGlass {
			data["break_glass"] = true
		}
		audit, auditArgs, err := auditlog.InsertFrom(auditlog.Event{
			OrgID: &p.OrgID, ActorUserID: &p.UserID, ActorKind: "user", Action: "session.start",
			TargetType: "resource", TargetID: p.ResourceID.String(), IP: p.ClientIP, Data: data,
		}, "s", "id", "session_id", len(args)+1)
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, startSession+audit+") SELECT id FROM s", append(args, auditArgs...)...).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotMember
		}
		return err
	})
	if errors.Is(err, errNotMember) {
		return uuid.Nil, err
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("session: start: %w", err)
	}
	return id, nil
}

// activeBreakGlass tells whether the user holds an active break-glass grant to the resource.
const activeBreakGlass = `SELECT EXISTS (SELECT 1 FROM access_grant WHERE org_id = $1 AND user_id = $2 AND resource_id = $3
	AND break_glass AND revoked_at IS NULL AND expires_at > now())`

// startSession inserts the session, only for a member of the organization (no
// row otherwise), and opens the CTE of its audit row, which the caller appends.
const startSession = `WITH s AS (
	INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip, kind, target, command, classes, paths)
	SELECT m.org_id, m.user_id, $3::uuid, $4::uuid, $5::uuid, $6::inet, $7::text, $8::text, NULLIF($9::text, ''), $10::text[], $11::text[]
	FROM membership m WHERE m.org_id = $1 AND m.user_id = $2
	RETURNING id
), audit AS (`

var errNotMember = fmt.Errorf("user not found: %w", apperr.ErrNotFound)

// End closes an open session of instanceID. A repeat call is a no-op; a
// session of another instance or organization is apperr.ErrNotFound. A
// non-nil c stores the final counters, never below the current ones. A non-nil
// r (tunnel passes one only with a command) stores the command, classes and
// paths of a console session, in the first close only.
func (s *Service) End(ctx context.Context, orgID, instanceID, id uuid.UUID, reason string, c *Counters, r *Result) error {
	if c == nil {
		c = &Counters{}
	}
	if r == nil {
		r = &Result{}
	}
	return dbx.InOrg(ctx, s.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		st := newStore(tx)
		res, err := st.EndSession(ctx, dbq.EndSessionParams{ID: id, OrgID: orgID, InstanceID: instanceID, Reason: reason,
			BytesUp: c.BytesUp, BytesDown: c.BytesDown, DatagramsUp: c.DatagramsUp, DatagramsDown: c.DatagramsDown,
			Command: r.Command, Classes: r.Classes, Paths: r.Paths})
		if errors.Is(err, pgx.ErrNoRows) {
			ok, err := st.SessionOfInstanceExists(ctx, dbq.SessionOfInstanceExistsParams{ID: id, OrgID: orgID, InstanceID: instanceID})
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("session not found: %w", apperr.ErrNotFound)
			}
			return nil
		}
		if err != nil {
			return err
		}
		return auditEnd(ctx, tx, "gateway", orgID, res, id, reason)
	})
}

// EndMissing closes the open sessions of instanceID that are absent from
// active and started over a minute ago, and returns how many it closed.
func (s *Service) EndMissing(ctx context.Context, orgID, instanceID uuid.UUID, active []uuid.UUID, reason string) (int64, error) {
	var n int64
	err := dbx.InOrg(ctx, s.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		rows, err := newStore(tx).EndMissingSessions(ctx, dbq.EndMissingSessionsParams{
			OrgID: orgID, InstanceID: instanceID, Active: active, Reason: reason})
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := auditEnd(ctx, tx, "gateway", orgID, r.ResourceID, r.ID, reason); err != nil {
				return err
			}
		}
		n = int64(len(rows))
		return nil
	})
	return n, err
}

// EndLost closes, across all organizations, the open sessions whose instance
// was deleted or has been silent for lostAfter.
func (s *Service) EndLost(ctx context.Context, lostAfter time.Duration) (int64, error) {
	var n int64
	err := dbx.System(ctx, s.pool, dbx.ReasonMaintenance, func(tx pgx.Tx) error {
		rows, err := newStore(tx).EndLostSessions(ctx, lostAfter.Seconds())
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := auditEnd(ctx, tx, "system", r.OrgID, r.ResourceID, r.ID, reasonGatewayLost); err != nil {
				return err
			}
		}
		n = int64(len(rows))
		return nil
	})
	return n, err
}

func auditEnd(ctx context.Context, tx pgx.Tx, actorKind string, orgID uuid.UUID, resourceID *uuid.UUID, id uuid.UUID, reason string) error {
	targetID := ""
	if resourceID != nil {
		targetID = resourceID.String()
	}
	return auditlog.Write(ctx, tx, auditlog.Event{
		OrgID: &orgID, ActorKind: actorKind, Action: "session.end",
		TargetType: "resource", TargetID: targetID,
		Data: map[string]any{"session_id": id.String(), "reason": reason},
	})
}

// UpdateCounters stores the totals of the open sessions of instanceID in one
// statement; a total never lowers the stored one, other sessions are ignored.
func (s *Service) UpdateCounters(ctx context.Context, orgID, instanceID uuid.UUID, cs []Counters) error {
	if len(cs) == 0 {
		return nil
	}
	arg := dbq.UpdateSessionCountersParams{OrgID: orgID, InstanceID: instanceID}
	for _, c := range cs {
		arg.Ids = append(arg.Ids, c.SessionID)
		arg.BytesUp = append(arg.BytesUp, c.BytesUp)
		arg.BytesDown = append(arg.BytesDown, c.BytesDown)
		arg.DatagramsUp = append(arg.DatagramsUp, c.DatagramsUp)
		arg.DatagramsDown = append(arg.DatagramsDown, c.DatagramsDown)
	}
	return dbx.InOrg(ctx, s.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		return newStore(tx).UpdateSessionCounters(ctx, arg)
	})
}
