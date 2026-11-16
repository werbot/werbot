// Package auditlog writes audit events in the transaction of the change.
package auditlog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Event is one audit record.
type Event struct {
	OrgID       *uuid.UUID
	ActorUserID *uuid.UUID
	ActorKind   string // "user" | "gateway" | "system"
	Action      string // "<domain>.<verb>", e.g. "member.invite"
	TargetType  string
	TargetID    string
	IP          netip.Addr     // zero value is stored as NULL
	Data        map[string]any // nil is stored as {}
}

var actorKinds = []string{"user", "gateway", "system"}

// Write inserts e into audit_event inside tx.
func Write(ctx context.Context, tx pgx.Tx, e Event) error {
	if e.Action == "" {
		return fmt.Errorf("auditlog: empty action")
	}
	if !slices.Contains(actorKinds, e.ActorKind) {
		return fmt.Errorf("auditlog: invalid actor kind %q", e.ActorKind)
	}
	data := e.Data
	if data == nil {
		data = map[string]any{}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("auditlog: marshal data: %w", err)
	}
	var ip *netip.Addr
	if e.IP.IsValid() {
		ip = &e.IP
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_event (org_id, actor_user_id, actor_kind, action, target_type, target_id, ip, data)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.OrgID, e.ActorUserID, e.ActorKind, e.Action, e.TargetType, e.TargetID, ip, raw)
	if err != nil {
		return fmt.Errorf("auditlog: insert: %w", err)
	}
	return nil
}
