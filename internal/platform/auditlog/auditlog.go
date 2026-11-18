// Package auditlog writes audit events in the transaction of the change.
package auditlog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

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

// columns is the audit_event insert of Write and InsertFrom.
const columns = "INSERT INTO audit_event (org_id, actor_user_id, actor_kind, action, target_type, target_id, ip, data)"

// Write inserts e into audit_event inside tx.
func Write(ctx context.Context, tx pgx.Tx, e Event) error {
	args, err := values(e)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, columns+" VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", args...); err != nil {
		return fmt.Errorf("auditlog: insert: %w", err)
	}
	return nil
}

// InsertFrom returns a statement that inserts e once for each row of from (a
// query name of the caller's statement, such as a CTE), with data[key] set to
// the text of that row's column. Its placeholders start at $first; args are
// their values. from and column are SQL identifiers and must be constants.
func InsertFrom(e Event, from, column, key string, first int) (sql string, args []any, err error) {
	if args, err = values(e); err != nil {
		return "", nil, err
	}
	p := make([]string, len(args)+1)
	for i := range p {
		p[i] = "$" + strconv.Itoa(first+i)
	}
	sql = columns + " SELECT " + strings.Join(p[:7], ", ") + ", " + p[7] + "::jsonb || jsonb_build_object(" + p[8] +
		"::text, " + from + "." + column + "::text) FROM " + from
	return sql, append(args, key), nil
}

// values checks e and returns the values of columns in order.
func values(e Event) ([]any, error) {
	if e.Action == "" {
		return nil, fmt.Errorf("auditlog: empty action")
	}
	if !slices.Contains(actorKinds, e.ActorKind) {
		return nil, fmt.Errorf("auditlog: invalid actor kind %q", e.ActorKind)
	}
	data := e.Data
	if data == nil {
		data = map[string]any{}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("auditlog: marshal data: %w", err)
	}
	var ip *netip.Addr
	if e.IP.IsValid() {
		ip = &e.IP
	}
	return []any{e.OrgID, e.ActorUserID, e.ActorKind, e.Action, e.TargetType, e.TargetID, ip, raw}, nil
}
