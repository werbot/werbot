// Package session records SSH sessions, each owned by one gateway instance.
package session

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	Kind, Target                                     string     // Kind "" is "terminal"
	Command                                          string     // exec only; "" is NULL
	Classes                                          []string   // exec only; nil is NULL
	Paths                                            []string   // nil is NULL
	AgentRunID                                       *uuid.UUID // an agent's session
	ToolCallID                                       *uuid.UUID // stored only when it is a tool call of AgentRunID
	// Guard runs in the start transaction before the insert; its error refuses the start.
	Guard func(ctx context.Context, tx pgx.Tx) error
}

// Result is what a session ran: for a console, the last checked command, its
// classes and the paths it touched; for an exec, its exit code and the SHA-256
// of its recorded output.
type Result struct {
	Command        string
	Classes, Paths []string
	ExitCode       *int32 // nil is NULL
	OutputSHA256   []byte // 32 bytes; nil is NULL
}

// Counters are a session's traffic totals since it began.
type Counters struct {
	SessionID                                      uuid.UUID
	BytesUp, BytesDown, DatagramsUp, DatagramsDown int64
}

// Denial is one refused connection attempt reported by a gateway.
type Denial struct {
	UserID                                         *uuid.UUID
	KeyFingerprint, Resource, Kind, Target, Reason string
	ClientIP                                       netip.Addr
	At                                             time.Time
	SessionID                                      *uuid.UUID // set for a command denied inside a session
	Classes, Paths, Reasons                        []string
	AgentRunID                                     *uuid.UUID // the agent run of the attempt
}
