package tunnel

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// SessionStart describes a session to record.
type SessionStart struct {
	OrgID, GatewayID, InstanceID, UserID, ResourceID uuid.UUID
	ClientIP                                         netip.Addr
	Kind, Target                                     string
	Front                                            string     // "ssh", "web" or empty: ssh
	Transport                                        string     // "tcp", "ws", "internal" or empty: tcp
	Command                                          string     // exec only
	Classes                                          []string   // exec only; nil when none
	Paths                                            []string   // the paths the command touches; nil when none
	AgentRunID, ToolCallID                           *uuid.UUID // an agent's session; nil for a person
	DBName                                           string     // the database a client asked for; "" when none
}

// SessionResult is what a session ran: for a console, the last checked
// command, its classes and paths; for an exec, its exit code and the SHA-256
// of its recorded output; for a querylog session, its query count; nil when
// the session has none. The field order is session.Result's.
type SessionResult struct {
	Command        string
	Classes, Paths []string
	ExitCode       *int32
	OutputSHA256   []byte
	Queries        int64
}

// Counters are a session's traffic totals since it began; up is client to target.
type Counters struct {
	SessionID                                      uuid.UUID
	BytesUp, BytesDown, DatagramsUp, DatagramsDown int64
}

// Denial is a connection attempt a gateway refused; UserID is nil when unknown.
// SessionID is set for a console line refused inside a session.
type Denial struct {
	UserID                                         *uuid.UUID
	KeyFingerprint, Resource, Kind, Target, Reason string
	ClientIP                                       netip.Addr
	At                                             time.Time
	SessionID                                      *uuid.UUID
	Classes, Paths, Reasons                        []string
	AgentRunID                                     *uuid.UUID // the agent run, when RecordDenial was not reachable
}
