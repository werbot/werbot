package gateway

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// windowChange is the payload of a window-change request.
func windowChange(cols, rows uint32) []byte {
	return gossh.Marshal(struct{ Cols, Rows, Width, Height uint32 }{cols, rows, 0, 0})
}

// An early window change is answered as gliderlabs answers it: true only with a PTY.
func TestEarlyWindowChangeReply(t *testing.T) {
	e := newSSHEnv(t, Config{})
	c := e.client(t, "alice+blocker")
	for _, pty := range []bool{true, false} {
		s, err := c.NewSession()
		require.NoError(t, err)
		if pty {
			require.NoError(t, s.RequestPty("xterm", 24, 80, gossh.TerminalModes{}))
		}
		ok, err := s.SendRequest("window-change", true, windowChange(100, 40))
		require.NoError(t, err)
		require.Equal(t, pty, ok, "pty=%v", pty)
		require.NoError(t, s.Close())
	}
}

// The last window change before shell is the initial window of the terminal.
func TestEarlyWindowSizeKept(t *testing.T) {
	e := newSSHEnv(t, Config{})
	s, err := e.client(t, "alice+spy").NewSession()
	require.NoError(t, err)
	require.NoError(t, s.RequestPty("xterm", 24, 80, gossh.TerminalModes{}))
	require.NoError(t, s.WindowChange(40, 100))
	var out bytes.Buffer
	s.Stdout = &out
	require.NoError(t, s.Shell())
	require.NoError(t, s.Wait())
	require.Contains(t, out.String(), "win=100x40")
}

// An early window change is parsed as gliderlabs parses it: cols and rows from
// the first 8 bytes, the pixel sizes optional.
func TestEarlyWindowChangePayload(t *testing.T) {
	e := newSSHEnv(t, Config{})
	c := e.client(t, "alice+spy")
	for _, tc := range []struct {
		payload []byte
		ok      bool
		win     string
	}{
		{windowChange(100, 40), true, "win=100x40"},
		{windowChange(100, 40)[:8], true, "win=100x40"},
		{windowChange(100, 40)[:7], false, "win=80x24"},
		{windowChange(0, 40), false, "win=80x24"},
		{windowChange(100, 0), false, "win=80x24"},
	} {
		s, err := c.NewSession()
		require.NoError(t, err)
		require.NoError(t, s.RequestPty("xterm", 24, 80, gossh.TerminalModes{}))
		ok, err := s.SendRequest("window-change", true, tc.payload)
		require.NoError(t, err)
		require.Equal(t, tc.ok, ok, "payload %x", tc.payload)
		var out bytes.Buffer
		s.Stdout = &out
		require.NoError(t, s.Shell())
		require.NoError(t, s.Wait())
		require.Contains(t, out.String(), tc.win, "payload %x", tc.payload)
	}
}

// fakeConnContext is the part of a connection ssh.Context a channelContext uses.
type fakeConnContext struct {
	ssh.Context // nil: only the methods below are called
	ctx         context.Context
	values      sync.Map
}

func (c *fakeConnContext) Deadline() (time.Time, bool) { return c.ctx.Deadline() }
func (c *fakeConnContext) Done() <-chan struct{}       { return c.ctx.Done() }
func (c *fakeConnContext) Err() error                  { return c.ctx.Err() }
func (c *fakeConnContext) SetValue(k, v any)           { c.values.Store(k, v) }
func (c *fakeConnContext) Value(k any) any {
	if v, ok := c.values.Load(k); ok {
		return v
	}
	return c.ctx.Value(k)
}

// A channel context resolves the connection's values, including ones set later,
// and has its own cancel cause.
func TestChannelContextValuesAndCause(t *testing.T) {
	connCtx, cancelConn := context.WithCancelCause(t.Context())
	conn := &fakeConnContext{ctx: connCtx}
	conn.SetValue(userKey{}, &authed{u: User{Login: "alice"}})
	c := newChannelContext(conn)
	c.SetValue(handshakeKey{}, "set later")
	require.Equal(t, "alice", handshakeUser(c).Login)
	require.Equal(t, "set later", conn.Value(handshakeKey{}))
	require.Equal(t, "set later", c.Value(handshakeKey{}))

	child, cancelChild := context.WithCancelCause(c)
	defer cancelChild(nil)
	c.cancel()
	require.ErrorIs(t, context.Cause(c), context.Canceled)
	require.ErrorIs(t, context.Cause(child), context.Canceled)
	require.NoError(t, connCtx.Err(), "the connection outlives its channel")

	errGone := errors.New("connection gone")
	c = newChannelContext(conn)
	cancelConn(errGone)
	require.ErrorIs(t, context.Cause(c), errGone)
}

type fakeNewChannel struct {
	gossh.NewChannel
	in chan *gossh.Request
}

func (f fakeNewChannel) Accept() (gossh.Channel, <-chan *gossh.Request, error) {
	return nil, f.in, nil
}

// After shell, a window change reaches gliderlabs only once the handler has read
// the PTY (gliderlabs writes the window into it), or when the channel ends.
func TestWindowChangeWaitsForPtyRead(t *testing.T) {
	for _, release := range []string{"pty read", "channel end"} {
		cctx := newChannelContext(&fakeConnContext{ctx: t.Context()})
		in := make(chan *gossh.Request)
		_, out, err := earlyWindowFilter{fakeNewChannel{in: in}, cctx}.Accept()
		require.NoError(t, err)

		in <- &gossh.Request{Type: "shell"}
		require.Equal(t, "shell", (<-out).Type, "other requests are not delayed")
		go func() { in <- &gossh.Request{Type: "window-change"} }()
		select {
		case <-out:
			t.Fatal("window change forwarded before the PTY was read")
		case <-time.After(100 * time.Millisecond):
		}
		if release == "pty read" {
			ptyRead(cctx)
			ptyRead(cctx) // once only
			require.Equal(t, "window-change", (<-out).Type)
		} else {
			cctx.cancel()
			require.Equal(t, "window-change", (<-out).Type)
		}
	}
}
