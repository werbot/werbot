package session

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/session/dbq"
)

// Deny audits refused attempts as "session.deny". The gateway is not trusted
// with identities: a user id that is no member of orgID is stored as nil. The
// target is the resource id when the name matches a resource, else the name;
// a set At is stored as "at" (RFC 3339, UTC).
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
			target, ok := ids[d.Resource]
			if !ok {
				target = d.Resource
			}
			actor := d.UserID
			if actor != nil && !slices.Contains(members, *actor) {
				actor = nil
			}
			data := map[string]any{"kind": d.Kind, "target": d.Target, "reason": d.Reason, "key_fingerprint": d.KeyFingerprint}
			if !d.At.IsZero() {
				data["at"] = d.At.UTC().Format(time.RFC3339) // when the gateway refused; created_at is when it arrived
			}
			if err := auditlog.Write(ctx, tx, auditlog.Event{
				OrgID: &orgID, ActorUserID: actor, ActorKind: "gateway", Action: "session.deny",
				TargetType: "resource", TargetID: target, IP: d.ClientIP, Data: data,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}
