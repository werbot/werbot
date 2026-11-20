package gateway

import (
	"errors"
	"io"
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
// context is the channel's, canceled when the client closes the channel or goes
// away (a target PTY ignores EOF); drain cancels the session through the
// pipeline, not through it.
func (h *sshHandler) serveSession(s ssh.Session) int {
	sctx := s.Context()
	pty, winch, isPty := s.Pty()
	ptyRead(sctx)
	resize := windowChanges(winch) // first: winch must drain on every path
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
	r := h.sessionRequest(s, resource, pty, isPty)
	defer session(sctx)() // from Open on: the idle limit never closes a wait for an approval
	stop := keepalive(sctx, h.keepalive)
	sess, err := h.p.Open(sctx, r)
	stop()
	if err != nil {
		say(openOut(r, s, err), openText(err, resource, r.Kind, h.p.ctl.Policy()))
		return 1
	}
	c, ok := LookupConnector(sess.Resource.Type)
	if !ok { // Open found it; registrations are never removed
		h.p.abort(sess)
		say(s, openText(ErrUnsupportedType, resource, r.Kind, h.p.ctl.Policy()))
		return 1
	}
	var exit atomic.Int32
	setExit := func(code int) { exit.Store(int32(code)) } //nolint:gosec // SSH exit statuses fit in 32 bits
	// the command the PDP decided, never the raw one again
	if r.Kind == KindTerminal {
		sess.Terminal = &Terminal{ReadWriter: s, Term: pty.Term, Command: sess.Command, Window: r.Window, Resize: resize, SetExitStatus: setExit}
	} else {
		sess.Exec = &Exec{Command: sess.Command, Stdin: s, Stdout: s, Stderr: s.Stderr(), SetExitStatus: setExit}
	}
	if err := h.p.Run(sctx, sess, c); err != nil {
		say(s, runText(err, resource))
		return 1
	}
	return int(exit.Load())
}

// sessionRequest is the OpenRequest of a session channel: a shell is a
// terminal, an exec with a PTY a terminal running the command, an exec without
// one an exec. Notice is the PTY of a terminal, stderr of an exec.
func (h *sshHandler) sessionRequest(s ssh.Session, resource string, pty ssh.Pty, isPty bool) OpenRequest {
	kind, w, notice := KindExec, Window{}, s.Stderr()
	if isPty || s.RawCommand() == "" {
		kind, notice = KindTerminal, s
		w = Window{Cols: pty.Window.Width, Rows: pty.Window.Height}
		if early := earlyWindow(s.Context()); isPty && early != nil {
			w = *early
		}
	}
	r := h.request(s.Context(), resource, kind, w, "")
	r.Command, r.Notice = s.RawCommand(), notice
	return r
}

// openOut is where the user reads the text of an Open error: Notice for a
// refused or timed-out command, the session channel otherwise.
func openOut(r OpenRequest, s ssh.Session, err error) io.Writer {
	if _, ok := errors.AsType[*CommandDeniedError](err); ok || errors.Is(err, ErrApprovalTimeout) {
		return r.Notice
	}
	return s
}

// windowChanges relays the window changes after the initial one, which comes
// with pty-req. It always drains winch, so the request loop never blocks; only
// the latest size waits for the connector. Once the channel closes, the result
// closes.
func windowChanges(winch <-chan ssh.Window) <-chan Window {
	out := make(chan Window, 1)
	if winch == nil {
		close(out)
		return out
	}
	go func() {
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
