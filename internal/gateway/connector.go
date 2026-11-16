package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"slices"
	"sync"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
)

// Kind is a session kind: what the front asked the connector to serve.
type Kind = connector.Kind

// Session kinds.
const (
	KindTerminal = connector.KindTerminal
	KindExec     = connector.KindExec
	KindStream   = connector.KindStream
)

// Connector serves the sessions of one resource type. It does not log: Serve
// returns its errors and the pipeline writes them. Nothing runs before the first Serve.
type Connector interface {
	Type() string
	Kinds() []Kind
	Serve(ctx context.Context, s *Session) error // ctx is canceled on terminate and drain
}

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

// Session is one opened session; the front fills Terminal, Exec or Stream by Kind.
type Session struct {
	ID        uuid.UUID
	Kind      Kind
	Front     string // "ssh"
	User      User
	Resource  Resource
	Secret    Secret
	ClientIP  netip.Addr
	Terminal  *Terminal                          // KindTerminal
	Exec      *Exec                              // KindExec
	Stream    io.ReadWriteCloser                 // KindStream; half-close through CloseWrite when the stream has it
	Target    string                             // KindStream: the requested host:port
	PinKey    func(alg string, key []byte) error // pins the target host key through the server; mismatch is ErrHostKeyMismatch
	Recording RecordingSink
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
