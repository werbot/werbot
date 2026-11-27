package gateway

import (
	"io"
	"net/netip"

	"github.com/google/uuid"
)

// OpenRequest is what a front asks the pipeline to open.
type OpenRequest struct {
	Front    string
	Kind     Kind
	Identity Identity
	Resource string // resource name
	Target   string // KindStream, KindDatagram: host:port
	ClientIP netip.Addr
	Window   Window    // KindTerminal: initial size for the recorder
	Command  string    // KindExec, KindTerminal with a command: the command to run
	Notice   io.Writer // the user's side channel: stderr of an exec, the PTY of a terminal
	// Handshake is the user the front authenticated for Identity; nil: none.
	Handshake *Handshake
	ToolCall  uuid.UUID // an agent's tool call (sshca.EnvToolCall); uuid.Nil: none
	MFA       MFAStep
	Transport string // "tcp" | "ws" | "internal" (the web terminal); "": tcp
	// RemoteForward: KindStream from tcpip-forward, Target "<bind_host>:<bind_port>".
	RemoteForward bool
	// Accept accepts the KindStream channel before StartSession, for a connector
	// with PreSession; nil: the front accepts it after Open.
	Accept func() (io.ReadWriteCloser, error)
}
