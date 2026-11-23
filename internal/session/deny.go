package session

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/session/dbq"
)

// Deny audits refused attempts as "session.deny", or as "session.command_denied"
// for a command refused inside a session. The gateway is not trusted
// with identities: a user id that is no member of orgID is stored as nil. The
// target is the resource id when the name matches a resource, else the name;
// a set At is stored as "at" (RFC 3339, UTC), a set AgentRunID as "agent_run_id".
func (s *Service) Deny(ctx context.Context, orgID uuid.UUID, ds []Denial) error {
	if len(ds) == 0 {
		return nil
	}
	names := make([]string, len(ds))
	var users []uuid.UUID
	for i, d := range ds {
		names[i] = d.Resource
		if d.UserID != nil {
			users = append(users, *d.UserID)
		}
	}
	return dbx.InOrg(ctx, s.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		st := newStore(tx)
		rows, err := st.ResourceIDsByName(ctx, dbq.ResourceIDsByNameParams{OrgID: orgID, Names: names})
		if err != nil {
			return err
		}
		ids := make(map[string]string, len(rows))
		for _, r := range rows {
			ids[r.Name] = r.ID.String()
		}
		members, err := st.MembersAmong(ctx, dbq.MembersAmongParams{OrgID: orgID, UserIds: users})
		if err != nil {
			return err
		}
		for _, d := range ds {
			if err := writeDenial(ctx, tx, st, orgID, d, ids, members); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeDenial audits one denial: "session.command_denied" for a command refused
// inside a session of orgID (skipped when the session is not one of orgID),
// else "session.deny".
func writeDenial(ctx context.Context, tx pgx.Tx, st Store, orgID uuid.UUID, d Denial, ids map[string]string, members []uuid.UUID) error {
	target, ok := ids[d.Resource]
	if !ok {
		target = d.Resource
	}
	actor := d.UserID
	if actor != nil && !slices.Contains(members, *actor) {
		actor = nil
	}
	e := auditlog.Event{OrgID: &orgID, ActorUserID: actor, ActorKind: "gateway", Action: "session.deny",
		TargetType: "resource", TargetID: target, IP: d.ClientIP}
	data := map[string]any{"kind": d.Kind, "target": d.Target, "reason": d.Reason, "key_fingerprint": d.KeyFingerprint}
	if d.SessionID != nil {
		own, err := st.SessionInOrg(ctx, dbq.SessionInOrgParams{ID: *d.SessionID, OrgID: orgID})
		if err != nil || !own {
			if err == nil {
				slog.WarnContext(ctx, "session: command denial for a foreign session skipped", "session_id", d.SessionID.String())
			}
			return err
		}
		e.Action, e.TargetType, e.TargetID = "session.command_denied", "session", d.SessionID.String()
		data = map[string]any{"command": d.Target, "classes": d.Classes, "paths": d.Paths, "reasons": d.Reasons, "reason": d.Reason}
	}
	e.Data = withDenialContext(data, d)
	return auditlog.Write(ctx, tx, e)
}

// withDenialContext adds to data when the gateway refused d and the agent run of d, when set.
func withDenialContext(data map[string]any, d Denial) map[string]any {
	if !d.At.IsZero() {
		data["at"] = d.At.UTC().Format(time.RFC3339) // when the gateway refused; created_at is when it arrived
	}
	if d.AgentRunID != nil {
		data["agent_run_id"] = d.AgentRunID.String()
	}
	return data
}
