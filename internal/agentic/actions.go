package agentic

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/agentic/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
)

// maxCommand bounds the command of a Denial in bytes, as the tunnel does.
const maxCommand = 1024

// alertClasses are the command classes whose denial emails agent_alert.
var alertClasses = []string{"secrets", "egress"}

var errNotSponsor = fmt.Errorf("agent run of another user: %w", apperr.ErrPermissionDenied)

// ActionInput is one action of a run: a session UserID starts with the run's token.
type ActionInput struct {
	OrgID, RunID, UserID uuid.UUID
}

// Denial is an action of a run the gateway refused by policy.
type Denial struct {
	Resource, Command string // Command at most 1024 bytes
	Classes, Reasons  []string
}

// CountAction counts one action of a run inside tx, the READ COMMITTED
// transaction that inserts the action's session after it. The run row is
// locked first, so parallel starts of one run on any replica count one at a
// time and never pass max_actions or max_concurrent open sessions. A run of
// another user, a frozen or locked agent is ErrPermissionDenied; a run that is
// not running or expired is ErrRunNotActive; a limit is ErrLimitExceeded. An
// allowed action resets the denial counter.
func (s *Service) CountAction(ctx context.Context, tx pgx.Tx, in ActionInput) error {
	st := newStore(tx)
	g, err := st.ActionGate(ctx, dbq.ActionGateParams{OrgID: in.OrgID, ID: in.RunID})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return errRunNotFound
	case err != nil:
		return err
	}
	if err := admit(g, in.UserID); err != nil {
		return err
	}
	// A statement of its own: it takes a new snapshot, which holds the
	// sessions of starts committed while this one waited for the run lock.
	open, err := st.OpenRunSessions(ctx, dbq.OpenRunSessionsParams{OrgID: in.OrgID, RunID: &in.RunID})
	if err != nil {
		return err
	}
	if open >= int64(g.MaxConcurrent) {
		return ErrLimitExceeded
	}
	return st.CountAction(ctx, dbq.CountActionParams{OrgID: in.OrgID, ID: in.RunID})
}

// admit refuses an action of g by userID before the open sessions are counted.
func admit(g dbq.ActionGateRow, userID uuid.UUID) error {
	if g.SponsorUserID != userID {
		return errNotSponsor
	}
	if err := gate(g.AgentStatus, g.Locked); err != nil {
		return err
	}
	if !g.Active {
		return ErrRunNotActive
	}
	if g.Actions >= g.MaxActions {
		return ErrLimitExceeded
	}
	return nil
}

// RecordDenial records a refused action of a run of orgID in one
// transaction: the denial counter grows and the PauseAfterDenials-th denial in
// a row pauses a running run (audit agent.run.pause); the denial is audited as
// session.deny by the sponsor; a secrets or egress class emails agent_alert to
// the sponsor and every owner and admin. paused reports the run's state after it.
func (s *Service) RecordDenial(ctx context.Context, orgID, runID uuid.UUID, d Denial) (paused bool, err error) {
	if len(d.Command) > maxCommand {
		return false, fmt.Errorf("command must be at most %d bytes: %w", maxCommand, apperr.ErrInvalidArgument)
	}
	err = s.tx(ctx, orgID, uuid.Nil, func(st Store, tx pgx.Tx) error {
		run, err := lockRun(ctx, st, orgID, runID)
		if err != nil {
			return err
		}
		if paused, err = countDenial(ctx, st, tx, run); err != nil {
			return err
		}
		if err := writeDenial(ctx, st, tx, run, d); err != nil {
			return err
		}
		return s.alert(ctx, st, tx, run, d)
	})
	return paused, err
}

// countDenial adds one denial to the locked run and reports whether it is paused.
func countDenial(ctx context.Context, st Store, tx pgx.Tx, run dbq.AgentRun) (bool, error) {
	n, status := run.DenialsInRow+1, run.Status
	pause := status == "running" && n >= PauseAfterDenials
	if pause {
		status = "paused"
	}
	if err := st.SetDenials(ctx, dbq.SetDenialsParams{OrgID: run.OrgID, ID: run.ID, DenialsInRow: n, Status: status}); err != nil {
		return false, err
	}
	if pause {
		if err := audit(ctx, tx, run.OrgID, "gateway", &run.SponsorUserID, "agent.run.pause", "agent_run", run.ID,
			map[string]any{"denials_in_row": n}); err != nil {
			return false, err
		}
	}
	return status == "paused", nil
}

// writeDenial audits d as session.writeDenial does: the target is the resource
// id when the name matches a resource of the organization, else the name.
// ponytail: a copy of session.writeDenial kept so count, audit and alert commit
// in one tx; route it through session.Deny once that atomicity is not needed.
func writeDenial(ctx context.Context, st Store, tx pgx.Tx, run dbq.AgentRun, d Denial) error {
	target := d.Resource
	id, err := st.ResourceIDByName(ctx, dbq.ResourceIDByNameParams{OrgID: run.OrgID, Name: d.Resource})
	switch {
	case err == nil:
		target = id.String()
	case !errors.Is(err, pgx.ErrNoRows):
		return err
	}
	return auditlog.Write(ctx, tx, auditlog.Event{OrgID: &run.OrgID, ActorUserID: &run.SponsorUserID, ActorKind: "gateway",
		Action: "session.deny", TargetType: "resource", TargetID: target,
		Data: map[string]any{"kind": "exec", "target": d.Command, "reason": "denied", "classes": d.Classes,
			"reasons": d.Reasons, "agent_run_id": run.ID}})
}

// alert queues agent_alert to the sponsor and every owner and admin, once per
// email, when d has a secrets or egress class.
func (s *Service) alert(ctx context.Context, st Store, tx pgx.Tx, run dbq.AgentRun, d Denial) error {
	if !slices.ContainsFunc(d.Classes, func(c string) bool { return slices.Contains(alertClasses, c) }) {
		return nil
	}
	names, err := st.AlertNames(ctx, dbq.AlertNamesParams{OrgID: run.OrgID, ID: run.ID})
	if err != nil {
		return err
	}
	to, err := st.AlertRecipients(ctx, dbq.AlertRecipientsParams{OrgID: run.OrgID, SponsorUserID: run.SponsorUserID})
	if err != nil {
		return err
	}
	data := map[string]string{"url": s.cfg.WebURL + "/agents/runs/" + run.ID.String(), "agent": names.Agent,
		"sponsor": names.Sponsor, "resource": d.Resource, "command": d.Command, "classes": strings.Join(d.Classes, ", ")}
	for _, email := range to {
		if err := s.emails.EnqueueTx(ctx, tx, email, "agent_alert", data); err != nil {
			return err
		}
	}
	return nil
}
