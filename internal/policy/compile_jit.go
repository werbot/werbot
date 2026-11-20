package policy

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// NoExpiry is the context.now bound of a lock without expiry.
const NoExpiry int64 = 9223372036854775807

// SnapshotGrant is an active just-in-time grant of one resource to one user.
type SnapshotGrant struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	ResourceID  uuid.UUID `json:"resource_id"`
	ExpiresUnix int64     `json:"expires_unix"`
}

// SnapshotLock is an active lock; ExpiresUnix 0 means until deleted.
type SnapshotLock struct {
	ID          uuid.UUID  `json:"id"`
	TargetType  string     `json:"target_type"`
	TargetID    *uuid.UUID `json:"target_id"`
	ExpiresUnix int64      `json:"expires_unix"`
}

// GrantSource lists the active grants of an organization.
type GrantSource interface {
	PolicyGrants(ctx context.Context, orgID uuid.UUID) ([]SnapshotGrant, error)
}

// LockSource lists the active locks of an organization.
type LockSource interface {
	PolicyLocks(ctx context.Context, orgID uuid.UUID) ([]SnapshotLock, error)
}

// Expirations returns the expiry times of the grants and locks of s, without
// 0, ascending and unique.
func (s Snapshot) Expirations() []int64 {
	var out []int64
	for _, g := range s.Grants {
		out = append(out, g.ExpiresUnix)
	}
	for _, l := range s.Locks {
		out = append(out, l.ExpiresUnix)
	}
	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) > 0 && out[0] == 0 {
		out = out[1:]
	}
	return out
}

// lockUntil is the context.now bound of l.
func lockUntil(l SnapshotLock) int64 {
	return cmp.Or(l.ExpiresUnix, NoExpiry)
}

// writeGrants emits one permit per grant, sorted by ID. A lock on a grant
// without expiry drops the grant; locks with expiry hold it until the last ends.
func writeGrants(b *strings.Builder, grants []SnapshotGrant, locks []SnapshotLock) {
	lockedUntil := map[uuid.UUID]int64{}
	for _, l := range locks {
		if l.TargetType == "grant" && l.TargetID != nil {
			lockedUntil[*l.TargetID] = max(lockedUntil[*l.TargetID], lockUntil(l))
		}
	}
	for _, g := range slices.SortedFunc(slices.Values(grants), func(x, y SnapshotGrant) int { return compareIDs(x.ID, y.ID) }) {
		until, locked := lockedUntil[g.ID]
		if until == NoExpiry {
			continue
		}
		fmt.Fprintf(b, "@id(\"grant:%s\")\npermit (principal == Werbot::User::%q, action, resource == Werbot::Resource::%q)\nwhen { context.now < %d",
			g.ID, g.UserID, g.ResourceID, g.ExpiresUnix)
		if locked {
			fmt.Fprintf(b, " && context.now >= %d", until)
		}
		b.WriteString(" };\n\n")
	}
}

// writeLocks emits one forbid per lock, sorted by ID. Grant locks are grant
// conditions; agent and MFA device targets are not compiled yet (stages 12, 14).
func writeLocks(b *strings.Builder, locks []SnapshotLock) error {
	for _, l := range slices.SortedFunc(slices.Values(locks), func(x, y SnapshotLock) int { return compareIDs(x.ID, y.ID) }) {
		switch l.TargetType {
		case "grant", "agent", "agent_run", "agents_of_user", "all_agents", "mfa_device":
			continue
		}
		if l.TargetID == nil {
			return fmt.Errorf("policy: lock %s: no target", l.ID)
		}
		scope, cond := "principal, action, resource", ""
		switch l.TargetType {
		case "user":
			scope = fmt.Sprintf("principal == Werbot::User::%q, action, resource", *l.TargetID)
		case "team":
			scope = fmt.Sprintf("principal in Werbot::Team::%q, action, resource", *l.TargetID)
		case "resource":
			scope = fmt.Sprintf("principal, action, resource == Werbot::Resource::%q", *l.TargetID)
		case "cli_session":
			cond = fmt.Sprintf("context.cli_session_id == %q && ", *l.TargetID)
		default:
			return fmt.Errorf("policy: lock %s: unknown target type %q", l.ID, l.TargetType)
		}
		fmt.Fprintf(b, "@id(\"lock:%s\")\nforbid (%s)\nwhen { %scontext.now < %d };\n\n", l.ID, scope, cond, lockUntil(l))
	}
	return nil
}
