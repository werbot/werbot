package connectcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Timings of the gateway connection; tests shrink them.
var (
	keepaliveInterval = 30 * time.Second // keepalive@openssh.com; 3 unanswered in a row drop the connection
	backoffFirst      = time.Second      // first wait before a redial, doubled up to backoffMax
	backoffMax        = 30 * time.Second
	reconnectWait     = 10 * time.Second // how long a new local connection waits for a redial
)

const (
	keepaliveMisses  = 3
	handshakeTimeout = 30 * time.Second
)

// errNotConnected is a local connection that waited reconnectWait in vain.
var errNotConnected = errors.New("the gateway is unreachable")

// link is the SSH connection of one resource. It is dialed again after a drop
// while the local listener stays open.
type link struct {
	addr     string
	conf     *ssh.ClientConfig
	resource string
	stderr   io.Writer

	mu     sync.Mutex
	client *ssh.Client   // nil while redialing
	up     chan struct{} // closed while client is set
}

func newLink(c *ssh.Client, addr string, conf *ssh.ClientConfig, resource string, stderr io.Writer) *link {
	up := make(chan struct{})
	close(up)
	return &link{addr: addr, conf: conf, resource: resource, stderr: stderr, client: c, up: up}
}

// dial opens an SSH connection to addr; a cancel of ctx aborts it.
func dial(ctx context.Context, addr string, conf *ssh.ClientConfig) (*ssh.Client, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, conf)
	if err != nil {
		_ = conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(c, chans, reqs), nil
}

// get is the connected client; while redialing it waits up to reconnectWait.
func (l *link) get(ctx context.Context) (*ssh.Client, error) {
	l.mu.Lock()
	c, up := l.client, l.up
	l.mu.Unlock()
	if c != nil {
		return c, nil
	}
	t := time.NewTimer(reconnectWait)
	defer t.Stop()
	select {
	case <-up:
	case <-t.C:
		return nil, errNotConnected
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.client == nil { // dropped again at once
		return nil, errNotConnected
	}
	return l.client, nil
}

// set installs c, or with nil starts a redial period.
func (l *link) set(c *ssh.Client) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.client = c
	if c != nil {
		close(l.up)
	} else {
		l.up = make(chan struct{})
	}
}

// run keeps the link up until ctx ends (nil) or a redial gets a final answer
// (that error). Every other redial failure is retried with backoff.
func (l *link) run(ctx context.Context) error {
	for {
		l.mu.Lock()
		c := l.client
		l.mu.Unlock()
		kctx, kstop := context.WithCancel(ctx)
		dropped := make(chan struct{})
		go func() { _ = c.Wait(); kstop(); close(dropped) }()
		keepalive(kctx, c)
		if ctx.Err() != nil {
			_ = c.Close()
			<-dropped
			return nil
		}
		<-dropped
		l.set(nil)
		_, _ = fmt.Fprintf(l.stderr, "%s: connection to the gateway lost; reconnecting\n", l.resource)
		c, err := l.redial(ctx)
		if c == nil {
			return err
		}
		l.set(c)
		_, _ = fmt.Fprintf(l.stderr, "%s: reconnected\n", l.resource)
	}
}

// redial dials until it succeeds, ctx ends (nil, nil) or the gateway gave a
// final answer (that error). Each new error is printed once.
func (l *link) redial(ctx context.Context) (*ssh.Client, error) {
	wait, last := backoffFirst, ""
	for {
		c, err := dial(ctx, l.addr, l.conf)
		switch {
		case err == nil:
			return c, nil
		case ctx.Err() != nil:
			return nil, nil
		case final(err):
			return nil, fmt.Errorf("%s: %w", l.resource, err)
		}
		if err.Error() != last {
			last = err.Error()
			_, _ = fmt.Fprintf(l.stderr, "%s: reconnect failed: %v; retrying\n", l.resource, err)
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, nil
		case <-t.C:
		}
		wait = min(2*wait, backoffMax)
	}
}

// final reports a dial error that a retry cannot fix: a changed host key or a
// refused key. The gateway checks keys also while it is not ready, so a refusal
// is its answer, not an outage; x/crypto has no sentinel error for it.
func final(err error) bool {
	return errors.Is(err, errHostKeyMismatch) || strings.Contains(err.Error(), "ssh: unable to authenticate")
}

// keepalive sends keepalive@openssh.com every keepaliveInterval until ctx ends
// and closes c when keepaliveMisses intervals in a row pass without a reply.
func keepalive(ctx context.Context, c *ssh.Client) {
	t := time.NewTicker(keepaliveInterval)
	defer t.Stop()
	reply := make(chan error, 1)
	pending, misses := false, 0
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-reply:
			pending, misses = false, 0
			if err != nil {
				_ = c.Close()
			}
		case <-t.C:
			if !pending {
				pending = true
				go func() { _, _, err := c.SendRequest("keepalive@openssh.com", true, nil); reply <- err }()
				continue
			}
			if misses++; misses >= keepaliveMisses {
				_ = c.Close()
			}
		}
	}
}
