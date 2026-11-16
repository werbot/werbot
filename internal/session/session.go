// Package session records SSH sessions, each owned by one gateway instance.
package session

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// Session lifetime timings.
const (
	EndLostInterval     = 30 * time.Second // how often the end_lost job runs
	EndLostAfter        = 2 * time.Minute  // an instance silent this long lost its sessions
	EndLostStartupGrace = 2 * time.Minute  // after a restart, instances need time to heartbeat again
)

const reasonGatewayLost = "gateway_lost"

// Start describes a session that has just begun.
type Start struct {
	OrgID, UserID, ResourceID, GatewayID, InstanceID uuid.UUID
	ClientIP                                         netip.Addr
}
