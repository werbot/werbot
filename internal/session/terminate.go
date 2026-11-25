package session

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/session/dbq"
)

// maxTerminateReason bounds a terminate reason in characters, as the proto rule.
const maxTerminateReason = 256

// ErrSessionEnded is a terminate request for a session that has already ended.
var ErrSessionEnded = fmt.Errorf("session already ended: %w", apperr.ErrFailedPrecondition)

// TerminateSession asks the gateway instance serving the open session id to
// end it. Owner and admin terminate any session of the organization, a member
// only their own; a session the principal may not see is apperr.ErrNotFound
// and an agent token is apperr.ErrPermissionDenied. The first request stores
// the reason and audits "session.terminate"; every request on the open
// session sends NOTIFY werbot_session_terminate to its instance.
func (s *Service) TerminateSession(ctx context.Context, p principal.Principal, id uuid.UUID, reason string) error {
	if p.IsAgent() {
		return fmt.Errorf("agents may not terminate sessions: %w", apperr.ErrPermissionDenied)
	}
	if n := utf8.RuneCountInString(reason); n == 0 || n > maxTerminateReason {
		return fmt.Errorf("terminate reason must have 1 to %d characters: %w", maxTerminateReason, apperr.ErrInvalidArgument)
	}
	arg := dbq.LockSessionToTerminateParams{ID: id, OrgID: p.OrgID}
	if !seesAll(p) {
		arg.UserID = &p.UserID
	}
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		return requestTerminate(ctx, tx, p, arg, reason)
	})
	if err == nil || errors.Is(err, apperr.ErrNotFound) || errors.Is(err, ErrSessionEnded) {
		return err
	}
	return fmt.Errorf("session: terminate: %w", err)
}

// requestTerminate locks the session of arg, records the first request with
// its audit event and notifies the session's instance.
func requestTerminate(ctx context.Context, tx pgx.Tx, p principal.Principal, arg dbq.LockSessionToTerminateParams, reason string) error {
	st := newStore(tx)
	row, err := st.LockSessionToTerminate(ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("session not found: %w", apperr.ErrNotFound)
	}
	if err != nil {
		return err
	}
	if row.EndedAt != nil {
		return ErrSessionEnded
	}
	if row.TerminateRequestedAt == nil {
		if err := st.RequestTerminate(ctx, dbq.RequestTerminateParams{ID: arg.ID, OrgID: arg.OrgID, Reason: reason}); err != nil {
			return err
		}
		if err := auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &p.OrgID, ActorUserID: &p.UserID, ActorKind: "user", Action: "session.terminate",
			TargetType: "session", TargetID: arg.ID.String(), IP: clientip.From(ctx), Data: map[string]any{"reason": reason},
		}); err != nil {
			return err
		}
	}
	if row.GatewayInstanceID == nil { // the instance is gone: nothing to notify
		return nil
	}
	return st.NotifySessionTerminate(ctx, row.GatewayInstanceID.String()+":"+arg.ID.String())
}

// PendingTerminations returns, ordered by id, the open sessions of instanceID
// with a terminate request: the tunnel resends them when a NOTIFY may be lost.
func (s *Service) PendingTerminations(ctx context.Context, orgID, instanceID uuid.UUID) (ids []uuid.UUID, err error) {
	err = dbx.ReadInOrg(s.pool, orgID, uuid.Nil, func(r *dbx.Read) (err error) {
		ids, err = newStore(r).PendingTerminations(ctx, dbq.PendingTerminationsParams{OrgID: orgID, InstanceID: instanceID})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("session: pending terminations: %w", err)
	}
	return ids, nil
}
