// Package audit serves the read-only event journal of an organization.
package audit

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// Event is one audit record. ActorUserID is nil for gateway and system events
// and once the user is deleted; ActorLogin is then empty. IP is the zero value when unknown.
type Event struct {
	ID                            uuid.UUID
	ActorUserID                   *uuid.UUID
	ActorLogin, ActorKind, Action string
	TargetType, TargetID          string
	IP                            netip.Addr
	Data                          map[string]any
	CreatedAt                     time.Time
}

// Filter narrows ListEvents.
type Filter struct {
	Action               string // "session" → session.*; "session.start" → exact match
	ActorUserID          *uuid.UUID
	TargetType, TargetID string
	From, To             *time.Time // [From, To) by created_at
	Page, PageSize       int        // Page 0 is 1; PageSize 0 is 50, above 200 is 200
}
