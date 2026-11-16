package gateway

import (
	"context"
	"sync/atomic"

	"github.com/gliderlabs/ssh"
)

// session serves a session channel and exits with the status of its command;
// returning closes the channel, which also ends the connector's input relay.
func (h *sshHandler) session(s ssh.Session) {
	_ = s.Exit(h.serveSession(s))
}

// serveSession maps the channel to a session kind: a shell is a terminal, an exec
// with a PTY a terminal running Command, an exec without one an exec. The session
// context comes from the connection's, canceled when the client goes away or, with
// a PTY, closes the channel (a target PTY ignores EOF); drain cancels the session
// through the pipeline, not through it.
func (h *sshHandler) serveSession(s ssh.Session) int {
	sctx := s.Context()
	ctx, cancel := context.WithCancel(sctx)
	defer cancel()
	pty, winch, isPty := s.Pty()
	resize := windowChanges(winch, cancel) // first: winch must drain on every path
	if handshakeUser(sctx) == nil {
		say(s, openText(h.p.reject(ErrNotReady), "", "", nil))
		return 1
	}
	_, resource, _ := ParseLogin(s.User()) // checked at the handshake
	if resource == "" {
		if !isPty || s.RawCommand() != "" {
			say(s, usageText)
			return 1
		}
		var code int
		if resource, code = h.menu(s, *handshakeUser(sctx)); resource == "" {
			return code
		}
	}
	kind, w := KindExec, Window{}
	if isPty || s.RawCommand() == "" {
		kind = KindTerminal
		w = Window{Cols: pty.Window.Width, Rows: pty.Window.Height}
	}
	sess, err := h.p.Open(ctx, h.request(sctx, resource, kind, w, ""))
	if err != nil {
		say(s, openText(err, resource, kind, h.p.ctl.Policy()))
		return 1
	}
	c, ok := LookupConnector(sess.Resource.Type)
	if !ok { // Open found it; registrations are never removed
		h.p.abort(sess)
		say(s, openText(ErrUnsupportedType, resource, kind, h.p.ctl.Policy()))
		return 1
	}
	var exit atomic.Int32
	setExit := func(code int) { exit.Store(int32(code)) } //nolint:gosec // SSH exit statuses fit in 32 bits
	if kind == KindTerminal {
		sess.Terminal = &Terminal{ReadWriter: s, Term: pty.Term, Command: s.RawCommand(), Window: w, Resize: resize, SetExitStatus: setExit}
	} else {
		sess.Exec = &Exec{Command: s.RawCommand(), Stdin: s, Stdout: s, Stderr: s.Stderr(), SetExitStatus: setExit}
	}
	if err := h.p.Run(ctx, sess, c); err != nil {
		say(s, runText(err, resource))
		return 1
	}
	return int(exit.Load())
}

// windowChanges relays the window changes after the initial one, which comes
// with pty-req. It always drains winch, so the request loop never blocks; only
// the latest size waits for the connector. Once the channel closes, the result
// closes and closed runs.
func windowChanges(winch <-chan ssh.Window, closed func()) <-chan Window {
	out := make(chan Window, 1)
	if winch == nil {
		close(out)
		return out
	}
	go func() {
		defer closed()
		defer close(out)
		<-winch // the initial window
		for w := range winch {
			select { // drop a size the connector has not taken yet
			case <-out:
			default:
			}
			out <- Window{Cols: w.Width, Rows: w.Height}
		}
	}()
	return out
}
