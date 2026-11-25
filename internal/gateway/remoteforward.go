package gateway

import (
	"cmp"
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// RemoteForward is a remote forward (ssh -R) of a KindStream session: the
// connector listens on BindHost:BindPort at the target, calls Ready once it
// does and hands each connection to Open.
type RemoteForward struct {
	BindHost string // as the client sent it: "", "localhost", "127.0.0.1" or "::1"
	BindPort uint32 // 1–65535
	// Open opens a forwarded-tcpip channel to the client for a connection
	// from originAddr:originPort at the target.
	Open  func(originAddr string, originPort uint32) (io.ReadWriteCloser, error)
	Ready func() // the connector calls it once the target listens
}

// Addr is the address the target listens on; an empty host is localhost.
func (rf RemoteForward) Addr() string {
	return net.JoinHostPort(cmp.Or(rf.BindHost, "localhost"), strconv.FormatUint(uint64(rf.BindPort), 10))
}

// Global requests of a remote forward (RFC 4254 §7.1).
const (
	tcpipForward       = "tcpip-forward"
	cancelTCPIPForward = "cancel-tcpip-forward"
)

// remoteForwardReady bounds the wait for the target to listen.
var remoteForwardReady = 10 * time.Second

// errForwardNotReady ends a remote forward whose target did not listen in time.
const errForwardNotReady endCause = "error"

// target checks the target of r on res: a remote forward's bind address, or
// a stream or datagram target (allowTarget); other kinds have none. The port
// is the one the PDP decides.
func (p *Pipeline) target(res Resource, r OpenRequest) (string, uint16, error) {
	switch {
	case r.RemoteForward:
		return bindTarget(res, r)
	case r.Kind == KindStream || r.Kind == KindDatagram:
		return allowTarget(res, r.Target)
	}
	return "", 0, nil
}

// bindTarget checks a remote forward: a stream on a resource that permits
// them, Target "<bind host>:<bind port from 1>". The front checked the host.
func bindTarget(res Resource, r OpenRequest) (string, uint16, error) {
	host, port, err := net.SplitHostPort(r.Target)
	n, perr := strconv.ParseUint(port, 10, 16)
	ep, ok, eerr := res.Endpoint()
	if err != nil || perr != nil || n == 0 || r.Kind != KindStream || eerr != nil || !ok || !ep.RemoteForward {
		return "", 0, ErrTargetNotAllowed
	}
	return host, uint16(n), nil
}

// authorize decides the connect request req of r (connect, with its second
// factor) and, for a remote forward, then the "<type>.remote_forward" action
// on the same request: a type without it is ErrUnsupportedKind.
func (p *Pipeline) authorize(ctx context.Context, pol *Policy, req *AccessRequest, r OpenRequest) error {
	if err := p.connect(ctx, pol, req, r); err != nil || !r.RemoteForward {
		return err
	}
	req.RemoteForward = true
	fr := remoteForwardOf(*req)
	if !pol.typeHasAction(fr.Resource.ID, fr.Action) {
		return ErrUnsupportedKind
	}
	if d := pol.Decide(fr); !d.Allow {
		return accessDenied{d.Reasons}
	}
	return nil
}

// remoteForwardOf is the remote_forward request of a session's request.
func remoteForwardOf(req AccessRequest) AccessRequest {
	req = connectOf(req)
	req.Action = req.Resource.Type + ".remote_forward"
	return req
}

// targetAllowed is the target check of a running session on res: a remote
// forward needs the resource to permit them, a stream or datagram its target.
func targetAllowed(res Resource, e evaluation) bool {
	if e.kind != KindStream && e.kind != KindDatagram {
		return true
	}
	ep, ok, err := res.Endpoint()
	if err != nil || !ok {
		return false
	}
	if e.req.RemoteForward {
		return ep.RemoteForward
	}
	return ep.Allow(res.Name, e.host, e.req.Port)
}

// forwardAllowed re-decides the remote_forward action of a remote forward.
func forwardAllowed(pol *Policy, req AccessRequest) bool {
	return !req.RemoteForward || pol.Decide(remoteForwardOf(req)).Allow
}

// bindAddr is the payload of tcpip-forward and cancel-tcpip-forward.
type bindAddr struct {
	Host string
	Port uint32
}

func (b bindAddr) String() string {
	return net.JoinHostPort(b.Host, strconv.FormatUint(uint64(b.Port), 10))
}

// forwardedTCPIP is the open data of a forwarded-tcpip channel (RFC 4254 §7.2).
type forwardedTCPIP struct {
	Addr       string
	Port       uint32
	OriginAddr string
	OriginPort uint32
}

// bindRequest is the bind address of a tcpip-forward the front serves: a
// direct login on a ready control plane, a loopback host (owner decision O3)
// and a port from 1.
func bindRequest(ctx ssh.Context, payload []byte) (resource string, b bindAddr, ok bool) {
	_, resource, _ = ParseLogin(ctx.User()) // checked at the handshake
	if err := gossh.Unmarshal(payload, &b); err != nil || resource == "" || handshakeUser(ctx) == nil {
		return "", b, false
	}
	switch b.Host {
	case "", "localhost", "127.0.0.1", "::1":
		return resource, b, b.Port > 0 && b.Port <= 65535
	}
	return "", b, false
}

// remoteForward serves tcpip-forward as a stream session with a
// RemoteForward: true once the target listens. Anything refused, a repeated
// address of the connection included, is false; a refusal by Open is a denial.
func (h *sshHandler) remoteForward(ctx ssh.Context, _ *ssh.Server, req *gossh.Request) (bool, []byte) {
	resource, b, ok := bindRequest(ctx, req.Payload)
	fs := forwardsOf(ctx)
	if !ok || fs.running(b.String()) {
		return false, nil
	}
	r := h.request(ctx, resource, KindStream, Window{}, RemoteForward{BindHost: b.Host, BindPort: b.Port}.Addr())
	r.RemoteForward = true
	sess, err := h.p.Open(ctx, r)
	if err != nil {
		return false, nil
	}
	c, ok := LookupConnector(sess.Resource.Type)
	if !ok { // Open found it; registrations are never removed
		h.p.abort(sess)
		return false, nil
	}
	return fs.run(ctx, h.p, sess, c, b), nil
}

// cancelRemoteForward serves cancel-tcpip-forward: the forward ends
// ("closed") before the answer; an unknown address is false.
func (h *sshHandler) cancelRemoteForward(ctx ssh.Context, _ *ssh.Server, req *gossh.Request) (bool, []byte) {
	var b bindAddr
	if gossh.Unmarshal(req.Payload, &b) != nil {
		return false, nil
	}
	fw := forwardsOf(ctx).take(b.String())
	if fw == nil {
		return false, nil
	}
	fw.cancel(nil)
	<-fw.ran
	return true, nil
}

// forwardsKey holds the *forwards of a connection.
type forwardsKey struct{}

// forwards are the remote forwards of one connection by bind address.
type forwards struct {
	mu sync.Mutex
	m  map[string]*forward
}

// forward is one running remote forward: cancel ends it, ran closes once Run returned.
type forward struct {
	cancel context.CancelCauseFunc
	ran    chan struct{}
}

// forwardsOf is the forwards of the connection of ctx. Global requests of a
// connection run one at a time, so the first one creates them.
func forwardsOf(ctx ssh.Context) *forwards {
	fs, _ := ctx.Value(forwardsKey{}).(*forwards)
	if fs == nil {
		fs = &forwards{m: map[string]*forward{}}
		ctx.SetValue(forwardsKey{}, fs)
	}
	return fs
}

func (fs *forwards) running(addr string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.m[addr] != nil
}

// take removes the forward of addr and returns it; nil: none.
func (fs *forwards) take(addr string) *forward {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fw := fs.m[addr]
	delete(fs.m, addr)
	return fw
}

// drop removes fw from addr and reports whether it was still there.
func (fs *forwards) drop(addr string, fw *forward) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.m[addr] != fw {
		return false
	}
	delete(fs.m, addr)
	return true
}

// run runs sess on c as the forward of b and reports whether the target
// listens: false when Run ends first or remoteForwardReady passes (the
// session ends with "error"). The forward holds the connection's idle limit.
// A forward the client did not end (cancel-tcpip-forward, the connection
// closed) closes the connection once it was ready, so the client learns it is gone.
func (fs *forwards) run(ctx ssh.Context, p *Pipeline, sess *Session, c Connector, b bindAddr) bool {
	conn, _ := ctx.Value(ssh.ContextKeyConn).(gossh.Conn)
	ready := make(chan struct{})
	var once sync.Once
	sess.RemoteForward = &RemoteForward{BindHost: b.Host, BindPort: b.Port, Ready: func() { once.Do(func() { close(ready) }) },
		Open: func(addr string, port uint32) (io.ReadWriteCloser, error) { return openForwarded(conn, b, addr, port) }}
	sctx, cancel := context.WithCancelCause(ctx)
	fw := &forward{cancel: cancel, ran: make(chan struct{})}
	fs.mu.Lock()
	fs.m[b.String()] = fw
	fs.mu.Unlock()
	done := session(ctx)
	go func() {
		defer close(fw.ran)
		defer done()
		_ = p.Run(sctx, sess, c) // the pipeline logs connector errors
		cancel(nil)
		if fs.drop(b.String(), fw) && closed(ready) && ctx.Err() == nil {
			_ = conn.Close()
		}
	}()
	select {
	case <-ready:
		return true
	case <-fw.ran:
	case <-time.After(remoteForwardReady):
	}
	fs.drop(b.String(), fw)
	cancel(errForwardNotReady)
	return false
}

// openForwarded opens a forwarded-tcpip channel for b on conn, from origin addr:port.
func openForwarded(conn gossh.Conn, b bindAddr, addr string, port uint32) (io.ReadWriteCloser, error) {
	ch, reqs, err := conn.OpenChannel("forwarded-tcpip", gossh.Marshal(forwardedTCPIP{b.Host, b.Port, addr, port}))
	if err != nil {
		return nil, err
	}
	go gossh.DiscardRequests(reqs)
	return ch, nil
}

// closed reports whether c is closed.
func closed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}
