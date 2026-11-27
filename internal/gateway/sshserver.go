package gateway

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/pires/go-proxyproto"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/sshdgram"
	"github.com/werbot/werbot/internal/platform/version"
)

// sshFront is the SSH entry: public key login, the resource menu, shell, exec,
// direct-tcpip, datagram@werbot.com and resource@werbot.com. One sshFront
// serves one pipeline, on any number of listeners.
type sshFront struct {
	cfg              Config
	hostKey          gossh.Signer
	log              *slog.Logger
	handshakeTimeout time.Duration // closes a connection that has not authenticated by then
	keepalive        time.Duration // keepalive@openssh.com period while a session waits to open

	once   sync.Once
	srv    *ssh.Server   // shared by every listener; built by the first Serve
	closed chan struct{} // closed once srv is closed, after the pipeline stopped
}

var _ Front = (*sshFront)(nil)

func newSSHFront(cfg Config, hostKey gossh.Signer, logger *slog.Logger) *sshFront {
	return &sshFront{cfg: cfg, hostKey: hostKey, log: logger, handshakeTimeout: 30 * time.Second, keepalive: 30 * time.Second}
}

func (*sshFront) Name() string { return "ssh" }

// Serve accepts SSH connections on ln until ctx ends, then closes ln. It may run
// on several listeners at once: they share one server. A failed listener closes
// only itself and its error returns at once; the others keep serving. Open
// connections keep running while the pipeline drains and are closed once it stops.
func (f *sshFront) Serve(ctx context.Context, ln net.Listener, p *Pipeline) error {
	srv := f.server(p)
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	select {
	case err := <-served: // srv.Serve closed ln
		if !errors.Is(err, ssh.ErrServerClosed) { // closed: the pipeline stopped
			return fmt.Errorf("ssh front: %w", err)
		}
	case <-ctx.Done():
		_ = ln.Close()
		<-served
	}
	<-f.closed
	return nil
}

// server is the server of f for p, built on first use. It closes once p
// stops, with the connections left in the menu or without a session.
func (f *sshFront) server(p *Pipeline) *ssh.Server {
	f.once.Do(func() {
		f.srv, f.closed = f.newServer(p), make(chan struct{})
		context.AfterFunc(p.stop, func() {
			_ = f.srv.Close()
			close(f.closed)
		})
	})
	return f.srv
}

func (f *sshFront) newServer(p *Pipeline) *ssh.Server {
	h := &sshHandler{p: p, log: f.log, keepalive: f.keepalive}
	srv := &ssh.Server{
		Version:          "Werbot_" + strings.ReplaceAll(version.Version, "-", "_"),
		PublicKeyHandler: h.auth,
		Handler:          h.session,
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"session": sessionChannel, "direct-tcpip": h.forward, sshdgram.ChannelType: h.forward,
		},
		RequestHandlers: map[string]ssh.RequestHandler{sshdgram.ResourceRequest: h.resourceInfo, tcpipForward: h.remoteForward, cancelTCPIPForward: h.cancelRemoteForward},
		ConnCallback: func(ctx ssh.Context, conn net.Conn) net.Conn {
			ctx.SetValue(transportKey{}, transportOf(conn))
			idle := newConnIdle(conn, f.cfg.IdleTimeout)
			ctx.SetValue(connIdleKey{}, idle)
			return newHandshakeConn(ctx, conn, f.handshakeTimeout, func() { idle.stop(); h.authFailed(ctx) })
		},
		ServerConfigCallback: h.verified,
	}
	srv.AddHostKey(f.hostKey)
	return srv
}

// wrapListener makes ln require a PROXY protocol header on every connection when proxyProtocol is set.
func wrapListener(ln net.Listener, proxyProtocol bool) net.Listener {
	if !proxyProtocol {
		return ln
	}
	return &proxyproto.Listener{Listener: ln, ReadHeaderTimeout: 5 * time.Second} // default policy: REQUIRE
}

// userKey holds the *authed record of the handshake acceptance; nil while the
// control plane was unreachable.
type userKey struct{}

// authed is the user accepted at the handshake, the identity it was found
// by, and when the lookup started.
type authed struct {
	u  User
	id Identity
	at time.Time
}

// refusedKey holds the *refusal of the last key auth refused; verifiedKey is set
// once a key signature is verified.
type (
	refusedKey  struct{}
	verifiedKey struct{}
)

type refusal struct {
	fingerprint, user string
	ip                netip.Addr
}

// authFailed reports one auth_failed denial for a connection that ends without a
// verified key after a key was refused; a port scan or a client that offered no
// key reports nothing.
func (h *sshHandler) authFailed(ctx ssh.Context) {
	r, _ := ctx.Value(refusedKey{}).(*refusal)
	if r == nil || ctx.Value(verifiedKey{}) != nil {
		return
	}
	_, resource, _ := ParseLogin(r.user)
	h.p.denials.add(newDenial("auth_failed", uuid.Nil, r.fingerprint, resource, "", "", r.ip))
}

type sshHandler struct {
	p         *Pipeline
	log       *slog.Logger
	keepalive time.Duration
}

// auth checks the key and the login. x/crypto also calls it for keys the client
// only offers, and the last call is for the key that signs, so the user is
// overwritten on every call: one found for an offered key never carries over
// (CVE-2024-45337). An unreachable control plane accepts the key without a user;
// the signature is still checked and every channel gets the not ready text.
func (h *sshHandler) auth(ctx ssh.Context, key ssh.PublicKey) bool {
	ctx.SetValue(userKey{}, (*authed)(nil))
	login, _, err := ParseLogin(ctx.User())
	if err != nil {
		return false
	}
	actx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ip := clientIP(ctx.RemoteAddr())
	start := h.p.now() // the reuse window counts from before the server's lookup
	id, err := keyIdentity(h.p.ctl.Policy(), login, key, start)
	var u User
	if err == nil {
		u, err = h.p.ctl.AuthenticateUser(actx, id, ip)
	}
	switch {
	case unreachable(err), errors.Is(err, ErrNotReady):
		return true
	case err != nil:
		lvl := slog.LevelWarn
		if c := connect.CodeOf(err); c == connect.CodeNotFound || c == connect.CodePermissionDenied || c == connect.CodeUnauthenticated {
			lvl = slog.LevelInfo // an unknown or refused key
		}
		h.log.Log(ctx, lvl, "ssh: key refused", "login", login, "client_ip", ip, "error", err)
		ctx.SetValue(refusedKey{}, &refusal{fingerprint: id.Value, user: ctx.User(), ip: ip})
		return false
	case u.Login != login:
		h.log.Info("ssh: key belongs to another login", "login", login, "client_ip", ip)
		ctx.SetValue(refusedKey{}, &refusal{fingerprint: id.Value, user: ctx.User(), ip: ip})
		return false
	}
	ctx.SetValue(userKey{}, &authed{u: u, id: id, at: start})
	return true
}

// handshakeUser is the user accepted at the handshake; nil means not ready.
func handshakeUser(ctx ssh.Context) *User {
	if a, _ := ctx.Value(userKey{}).(*authed); a != nil {
		return &a.u
	}
	return nil
}

// forward serves direct-tcpip as a stream and datagram@werbot.com as a datagram
// session; both carry the same open data. The channel is accepted only after
// Open succeeded, so an unreachable target closes an accepted channel and a
// refusal is Prohibited. A connector with PreSession has Open accept the
// stream before StartSession: a refusal after that closes the channel.
func (h *sshHandler) forward(_ *ssh.Server, conn *gossh.ServerConn, nc gossh.NewChannel, ctx ssh.Context) {
	kind := KindStream
	if nc.ChannelType() == sshdgram.ChannelType {
		kind = KindDatagram
	}
	var d sshdgram.OpenData
	if err := gossh.Unmarshal(nc.ExtraData(), &d); err != nil {
		_ = nc.Reject(gossh.ConnectionFailed, "invalid "+nc.ChannelType()+" request")
		return
	}
	_, resource, _ := ParseLogin(conn.User()) // checked at the handshake
	switch {
	case handshakeUser(ctx) == nil:
		_ = nc.Reject(gossh.Prohibited, openText(h.p.reject(ErrNotReady), resource, kind, nil))
		return
	case resource == "":
		_ = nc.Reject(gossh.Prohibited, usageText)
		return
	}
	ec := &earlyChannel{nc: nc}
	r := h.request(ctx, resource, kind, Window{}, net.JoinHostPort(d.Host, strconv.FormatUint(uint64(d.Port), 10)))
	r.Accept = ec.accept
	sess, err := h.p.Open(ctx, r)
	if err != nil {
		ec.reject(openText(err, resource, kind, h.p.ctl.Policy()))
		return
	}
	c, ok := LookupConnector(sess.Resource.Type)
	if !ok { // Open found it; registrations are never removed
		h.p.abort(sess)
		ec.reject(openText(ErrUnsupportedType, resource, kind, h.p.ctl.Policy()))
		return
	}
	ch, err := ec.open()
	if err != nil {
		h.p.abort(sess)
		return
	}
	defer ch.Close() //nolint:errcheck // the client may be gone
	attach(sess, kind, ch)
	defer session(ctx)()
	_ = h.p.Run(ctx, sess, c) // the pipeline logs connector errors
}

// request is the OpenRequest of the key that signed the handshake; a
// certificate keeps the identity its handshake verified.
func (h *sshHandler) request(ctx ssh.Context, resource string, kind Kind, w Window, target string) OpenRequest {
	key, _ := ctx.Value(ssh.ContextKeyPublicKey).(ssh.PublicKey) // set by the last accepted key: the one that signed
	fp := ""
	if key != nil {
		fp = gossh.FingerprintSHA256(key)
	}
	r := OpenRequest{Front: "ssh", Kind: kind, Identity: Identity{Method: "ssh-key", Value: fp},
		Resource: resource, Target: target, ClientIP: clientIP(ctx.RemoteAddr()), Window: w}
	if a, _ := ctx.Value(userKey{}).(*authed); a != nil && a.id.Value == fp {
		r.Identity, r.Handshake = a.id, &Handshake{User: a.u, At: a.at}
	}
	r.Transport, _ = ctx.Value(transportKey{}).(string)
	r.MFA.Method, _ = ctx.Value(mfaMethodKey{}).(string)
	sid, _ := ctx.Value(ssh.ContextKeySessionID).(string) // hex, set with the connection metadata
	r.MFA.Session, _ = hex.DecodeString(sid)
	return r
}

// clientIP is the client address; with the PROXY protocol, the one from the header.
func clientIP(a net.Addr) netip.Addr {
	if t, ok := a.(*net.TCPAddr); ok {
		return t.AddrPort().Addr().Unmap()
	}
	return netip.Addr{}
}
