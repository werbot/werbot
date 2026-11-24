package gateway

import (
	"context"
	"errors"
	"io"
	"sync/atomic"

	"github.com/gliderlabs/ssh"
)

// session serves a session channel and exits with the status of its command;
// returning closes the channel, which also ends the connector's input relay.
// A session the gateway ended has no status: the channel closes first, so the
// library's Exit(0) after the handler sends nothing and the client never reads
// a command cut short as a success.
func (h *sshHandler) session(s ssh.Session) {
	if code := h.serveSession(s); code >= 0 {
		_ = s.Exit(code)
		return
	}
	_ = s.Close()
}

// ended is the end reason of s served under ctx; s is cut when the gateway
// ended it with an end cause, not when it closed or failed on its own.
func (s *Session) ended(ctx context.Context, err error) string {
	s.cut = errors.As(context.Cause(ctx), new(endCause))
	return endReason(ctx, err)
}

// exitStatus is the status of the command of s, -1 (none) when s was cut.
func exitStatus(s *Session, code int32) int {
	if s.cut {
		return -1
	}
	return int(code)
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
	h.withKeepalive(sctx, sess)
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
	return exitStatus(sess, exit.Load())
}

// withKeepalive sends keepalives on the connection of ctx while a console
// line of s is checked: an approval wait inside Run carries no data either.
func (h *sshHandler) withKeepalive(ctx ssh.Context, s *Session) {
	check := s.CheckCommand
	if check == nil {
		return
	}
	s.CheckCommand = func(c context.Context, command, cwd, home string, notice io.Writer) (CommandCheck, error) {
		defer keepalive(ctx, h.keepalive)()
		return check(c, command, cwd, home, notice)
	}
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
	r.Command, r.Notice, r.ToolCall = s.RawCommand(), notice, toolCallOf(s.Environ())
	r.MFA.Prompt = noticePrompt(notice)
	if kind == KindTerminal {
		r.MFA.Prompt = terminalPrompt(s)
	}
	return r
}

// openOut is where the user reads the text of an Open error: Notice for a
// refused or timed-out command and a refused agent run, the session channel
// otherwise.
func openOut(r OpenRequest, s ssh.Session, err error) io.Writer {
	if noticed(err) {
		return r.Notice
	}
	return s
}

// noticed reports whether the user reads the text of an Open error on Notice.
func noticed(err error) bool {
	_, denied := errors.AsType[*CommandDeniedError](err)
	return denied || errors.Is(err, ErrApprovalTimeout) || errors.Is(err, ErrAgentLimit) || errors.Is(err, ErrAgentRunNotActive)
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
