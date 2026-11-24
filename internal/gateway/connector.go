package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// Kind is a session kind: what the front asked the connector to serve.
type Kind = connector.Kind

// Session kinds.
const (
	KindTerminal = connector.KindTerminal
	KindExec     = connector.KindExec
	KindStream   = connector.KindStream
	KindDatagram = connector.KindDatagram
)

// Datagrams are the datagrams of one session. flow numbers a client flow (its
// source address on the client): a DNS client sends each query from a new port,
// and the answers must go back to their flow.
type Datagrams interface {
	Recv(buf []byte) (flow uint32, n int, err error) // one datagram from the client
	Send(flow uint32, p []byte) error                // one datagram to the client
	Close() error
}

// Connector serves the sessions of one resource type. It does not log: Serve
// returns its errors and the pipeline writes them. Nothing runs before the first Serve.
type Connector interface {
	Type() string
	Kinds() []Kind
	Serve(ctx context.Context, s *Session) error // ctx is canceled on terminate and drain
}

// TargetDialer marks a connector that reaches its target through Session.Dial
// (telnet, tcp, udp). Open checks the host it dials against Config.DenyLocalTargets
// before StartSession, so a refusal is a target_not_allowed denial, not a failed session.
type TargetDialer interface{ DialsTarget() }

var (
	connMu   sync.RWMutex
	connRegs = map[string]Connector{}
)

// RegisterConnector adds c; it panics on a duplicate type or empty Kinds.
func RegisterConnector(c Connector) {
	if len(c.Kinds()) == 0 {
		panic(fmt.Sprintf("gateway: connector %q has no kinds", c.Type()))
	}
	connMu.Lock()
	defer connMu.Unlock()
	if _, dup := connRegs[c.Type()]; dup {
		panic(fmt.Sprintf("gateway: connector %q already registered", c.Type()))
	}
	connRegs[c.Type()] = c
}

// LookupConnector returns the connector of a resource type.
func LookupConnector(typ string) (Connector, bool) {
	connMu.RLock()
	defer connMu.RUnlock()
	c, ok := connRegs[typ]
	return c, ok
}

// connectors lists the registered types in ascending order, for Enroll and heartbeats.
func connectors() []string {
	connMu.RLock()
	defer connMu.RUnlock()
	out := make([]string, 0, len(connRegs))
	for t := range connRegs {
		out = append(out, t)
	}
	slices.Sort(out)
	return out
}

// Session is one opened session; the front fills Terminal, Exec, Stream or Datagram by Kind.
type Session struct {
	ID       uuid.UUID
	Kind     Kind
	Front    string // "ssh"
	User     User
	Resource Resource
	Secret   Secret
	ClientIP netip.Addr
	Terminal *Terminal                          // KindTerminal
	Exec     *Exec                              // KindExec
	Stream   io.ReadWriteCloser                 // KindStream; half-close through CloseWrite when the stream has it
	Datagram Datagrams                          // KindDatagram
	Target   string                             // KindStream, KindDatagram: the requested host:port
	PinKey   func(alg string, key []byte) error // pins the target host key through the server unless the snapshot pins it; mismatch is ErrHostKeyMismatch
	// Command is what the connector runs (KindExec, KindTerminal with a command):
	// the command the PDP decided, or the request's when the type runs none.
	// The console sets it to the last line it ran.
	Command string
	// Classes and Paths are those of Command; the console sets the classes
	// and paths of every line it ran, sorted and unique.
	Classes, Paths []string
	// Console marks a shell served by ConsoleServer: CheckCommand checks a
	// line (command) run in cwd, with home resolving "~", telling the user
	// about an approval wait on notice; nil for any other session.
	Console      bool
	CheckCommand func(ctx context.Context, command, cwd, home string, notice io.Writer) (CommandCheck, error)
	// RemoteForward is the remote forward (ssh -R) of a KindStream session
	// from tcpip-forward; Stream is nil.
	RemoteForward *RemoteForward
	// Dial connects to the target; on the embedded gateway it refuses local
	// addresses with ErrTargetNotAllowed.
	Dial      func(ctx context.Context, network, address string) (net.Conn, error)
	Recording RecordingSink     // a no-op sink for kinds the type does not record
	Metrics   *metrics.Registry // for connector counters; nil means a private registry

	outcome *execOutcome // an exec's exit status and output hash, kept by Run
	cut     bool         // the gateway ended it (sweeper, lock, cancel, drain), set by Run
}

// Redact removes the credential value from text bound for the log.
func (s Secret) Redact(text string) string {
	if len(s.Value) == 0 {
		return text
	}
	return strings.ReplaceAll(text, string(s.Value), "[redacted]")
}

// Window is a terminal size.
type Window struct{ Cols, Rows int }

// Terminal is an interactive session.
type Terminal struct {
	io.ReadWriter               // user input / output to the user
	Term          string        // terminal type from pty-req; "" means no PTY
	Command       string        // "" is a shell; otherwise a command with a PTY
	Window        Window        // initial size
	Resize        <-chan Window // later size changes
	SetExitStatus func(code int)
}

// Exec is a command without a PTY.
type Exec struct {
	Command        string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	SetExitStatus  func(code int)
}

// Front accepts users on a listener and hands their sessions to the pipeline.
type Front interface {
	Name() string
	Serve(ctx context.Context, ln net.Listener, p *Pipeline) error
}
