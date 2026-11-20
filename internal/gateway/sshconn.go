package gateway

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// handshakeKey holds the *time.Timer that closes a connection still unauthenticated.
type handshakeKey struct{}

// handshakeConn is closed by its timer unless authentication stops it first;
// gliderlabs resets conn deadlines on every read, so a deadline cannot do this.
// Close stops the timer, so a failed or finished connection releases it at once,
// and runs closed once: x/crypto closes the conn when the handshake fails.
type handshakeConn struct {
	net.Conn
	timer  *time.Timer
	closed func() // nil: none
	once   sync.Once
}

func newHandshakeConn(ctx ssh.Context, conn net.Conn, timeout time.Duration, closed func()) net.Conn {
	c := &handshakeConn{Conn: conn, closed: closed}
	c.timer = time.AfterFunc(timeout, func() { _ = conn.Close() })
	ctx.SetValue(handshakeKey{}, c.timer)
	return c
}

func (c *handshakeConn) Close() error {
	c.timer.Stop()
	if c.closed != nil {
		c.once.Do(c.closed)
	}
	return c.Conn.Close()
}

// connIdleKey holds the *connIdle of a connection.
type connIdleKey struct{}

// connIdle closes a connection that runs no session for d (Config.IdleTimeout),
// keepalives or not; menu input restarts it. A running session keeps it open: the sweeper ends idle
// sessions by their resource's idle_timeout, which may be longer than d.
type connIdle struct {
	mu      sync.Mutex
	d       time.Duration
	timer   *time.Timer // nil: no limit
	running int
	closed  bool
}

func newConnIdle(conn net.Conn, d time.Duration) *connIdle {
	c := &connIdle{d: d}
	if d > 0 {
		c.timer = time.AfterFunc(d, func() { _ = conn.Close() })
	}
	return c
}

// session marks a session running on the connection of ctx until the returned
// func is called.
func session(ctx ssh.Context) (done func()) {
	c, _ := ctx.Value(connIdleKey{}).(*connIdle)
	if c == nil || c.timer == nil {
		return func() {}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running++
	c.timer.Stop()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.running--; c.running == 0 && !c.closed {
			c.timer.Reset(c.d)
		}
	}
}

// keepalive sends keepalive@openssh.com on the connection of ctx every d until
// stop is called or the channel ends: a connection that waits for an approval
// carries no data, and NATs or load balancers may drop it. d <= 0 sends none.
func keepalive(ctx ssh.Context, d time.Duration) (stop func()) {
	conn, _ := ctx.Value(ssh.ContextKeyConn).(gossh.Conn)
	if conn == nil || d <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(d)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if _, _, err := conn.SendRequest("keepalive@openssh.com", true, nil); err != nil {
					return
				}
			}
		}
	}()
	return func() { close(done) }
}

// touch restarts the idle limit of the connection of ctx while it runs no
// session: input in the resource menu is activity.
func touch(ctx ssh.Context) {
	c, _ := ctx.Value(connIdleKey{}).(*connIdle)
	if c == nil || c.timer == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running == 0 && !c.closed {
		c.timer.Reset(c.d)
	}
}

// stop releases the timer once the connection is closed.
func (c *connIdle) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.timer != nil {
		c.timer.Stop()
	}
}

// authenticated stops the handshake timer once a signature is verified for a
// user. A connection the control plane could not check (no user) keeps the
// timer: every channel gets only the not ready text, so the limit closes it.
func authenticated(ctx ssh.Context) *gossh.ServerConfig {
	return &gossh.ServerConfig{VerifiedPublicKeyCallback: func(_ gossh.ConnMetadata, _ gossh.PublicKey, perms *gossh.Permissions, _ string) (*gossh.Permissions, error) {
		ctx.SetValue(verifiedKey{}, true)
		if t, ok := ctx.Value(handshakeKey{}).(*time.Timer); ok && handshakeUser(ctx) != nil {
			t.Stop()
		}
		return perms, nil
	}}
}

// channelContext is the context of one session channel: the connection's values
// and metadata, canceled when the channel closes or the connection ends.
type channelContext struct {
	ssh.Context
	done   context.Context
	cancel context.CancelFunc
	window atomic.Pointer[Window] // the last window change before shell or exec

	ptyTaken     chan struct{} // closed once the handler has read the session's PTY
	ptyTakenOnce sync.Once
}

func newChannelContext(ctx ssh.Context) *channelContext {
	c := &channelContext{Context: ctx, ptyTaken: make(chan struct{})}
	c.done, c.cancel = context.WithCancel(ctx)
	return c
}

func (c *channelContext) Deadline() (time.Time, bool) { return c.done.Deadline() }
func (c *channelContext) Done() <-chan struct{}       { return c.done.Done() }
func (c *channelContext) Err() error                  { return c.done.Err() }

// Value goes through the channel's cancel context, so a derived context finds
// it (context.Cause is per channel); other keys reach the connection's values,
// SetValue included.
func (c *channelContext) Value(k any) any { return c.done.Value(k) }

// earlyWindow is the last window change sent before shell or exec, nil when none.
func earlyWindow(ctx ssh.Context) *Window {
	if c, ok := ctx.(*channelContext); ok {
		return c.window.Load()
	}
	return nil
}

// ptyRead tells the channel's relay that the handler has read the session's PTY,
// so later window changes may reach gliderlabs, which writes them into it.
func ptyRead(ctx ssh.Context) {
	if c, ok := ctx.(*channelContext); ok {
		c.ptyTakenOnce.Do(func() { close(c.ptyTaken) })
	}
}

// sessionChannel serves a session channel with gliderlabs, minus window changes
// sent before shell or exec. gliderlabs buffers one window (the pty-req one) and
// blocks its request loop on the next until our handler drains them; the handler
// starts only on shell or exec, so an early window change would hang the channel
// forever, even after the connection closes. The session gets a context canceled
// when its channel closes, with or without a PTY.
func sessionChannel(srv *ssh.Server, conn *gossh.ServerConn, nc gossh.NewChannel, ctx ssh.Context) {
	cctx := newChannelContext(ctx)
	ssh.DefaultSessionHandler(srv, conn, earlyWindowFilter{nc, cctx}, cctx)
}

type earlyWindowFilter struct {
	gossh.NewChannel
	ctx *channelContext
}

// Accept relays the channel requests. Until a shell or exec passes, a window
// change is answered here as gliderlabs would (true only after a pty-req), and
// its size is kept as the initial window. After shell or exec a window change
// waits until the handler has read the PTY (ptyRead) or the channel ends: gliderlabs
// writes the window into the PTY the handler copies. The relay ends, and cancels the
// channel context, when the channel closes.
func (f earlyWindowFilter) Accept() (gossh.Channel, <-chan *gossh.Request, error) {
	ch, in, err := f.NewChannel.Accept()
	if err != nil {
		f.ctx.cancel()
		return ch, in, err
	}
	out := make(chan *gossh.Request)
	go func() {
		defer close(out)
		defer f.ctx.cancel() // x/crypto closes in on channel close and connection end
		started, sawPty := false, false
		for req := range in {
			switch {
			case started:
				if req.Type == "window-change" {
					select {
					case <-f.ctx.ptyTaken:
					case <-f.ctx.Done():
					}
				}
			case req.Type == "window-change":
				f.earlyWindowChange(req, sawPty, out)
				continue
			case req.Type == "pty-req":
				sawPty = true
			case req.Type == "shell" || req.Type == "exec":
				started = true
			}
			out <- req
		}
	}()
	return ch, out, nil
}

// earlyWindowChange keeps the size of a window change sent before shell or exec
// and answers it. Like gliderlabs, it reads cols and rows from the first 8
// bytes. Replies go in request order, so before replying it hands gliderlabs a
// sentinel: once gliderlabs takes it, every earlier request is
// answered (the sentinel wants no reply, so gliderlabs answers it with nothing).
func (f earlyWindowFilter) earlyWindowChange(req *gossh.Request, sawPty bool, out chan<- *gossh.Request) {
	var cols, rows uint32
	if p := req.Payload; len(p) >= 8 {
		cols, rows = binary.BigEndian.Uint32(p), binary.BigEndian.Uint32(p[4:])
	}
	ok := sawPty && cols > 0 && rows > 0
	if ok {
		f.ctx.window.Store(&Window{Cols: int(cols), Rows: int(rows)})
	}
	if req.WantReply {
		out <- &gossh.Request{Type: "werbot-sync"}
		_ = req.Reply(ok, nil)
	}
}
