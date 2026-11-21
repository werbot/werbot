package gateway

import (
	"context"
	"errors"
	"io"
	"slices"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway/recorder"
)

// Errors of an interactive shell.
var (
	// ErrShellDenied refuses an interactive shell; the front reads
	// "Interactive shell is not allowed on <resource>".
	ErrShellDenied = errors.New("interactive shell is not allowed")
	// ErrConsoleUnavailable ends a console that cannot read the working
	// directory on the target; the user reads its text.
	ErrConsoleUnavailable = errors.New("Console unavailable: cannot read working directory") //nolint:staticcheck // the user reads this text
)

// ConsoleServer is a connector that serves a shell under the console
// obligation as the command console: every line is checked through
// Session.CheckCommand before it runs.
type ConsoleServer interface {
	ServeConsole(ctx context.Context, s *Session) error
}

// CommandCheck is an allowed console line: its classes and paths
// (shellclass.PathRef texts).
type CommandCheck struct {
	Classes, Paths []string
}

// authorizeShell takes the shell decision of a terminal on a type with a
// shell action. A shell passes on a plain allow and comes back as the shell
// request the sweeper re-decides; under exactly the console obligation, on a
// type the console can serve, it comes back with Console set. A terminal with
// a command passes only on a plain allow and keeps its exec request (Ruling
// O1). Anything else is ErrShellDenied.
func (p *Pipeline) authorizeShell(pol *Policy, req AccessRequest, r OpenRequest, typ connector.Type) (AccessRequest, error) {
	action, ok := typ.ShellAction()
	if r.Kind != KindTerminal || !ok {
		return req, nil
	}
	shell := connectOf(req)
	shell.Action = action
	d := pol.Decide(shell)
	switch {
	case d.Allow && r.Command != "":
		return req, nil
	case d.Allow:
		return shell, nil
	case r.Command == "" && slices.Equal(d.Obligations, []string{ObligationConsole}) && consoleServed(typ):
		shell.Console = true
		return shell, nil
	}
	return req, ErrShellDenied
}

// consoleServed reports whether the console can serve typ: it runs every line
// on the exec action, and the connector implements ConsoleServer.
func consoleServed(typ connector.Type) bool {
	_, exec := typ.ExecAction()
	c, _ := LookupConnector(typ.Name)
	_, ok := c.(ConsoleServer)
	return exec && ok
}

// commandChecker is Session.CheckCommand of a console session req (nil for any
// other): each call analyses and decides one line from its cwd and home on
// the exec action on the current snapshot, waits for an approval when one is
// asked, and reports a refusal with the session id. The session does not
// idle out during the call.
func (p *Pipeline) commandChecker(req AccessRequest, r OpenRequest, s *Session) func(context.Context, string, string, string, io.Writer) (CommandCheck, error) {
	if !req.Console {
		return nil
	}
	typ, _ := connector.Lookup(req.Resource.Type)
	base := connectOf(req)
	base.Action, _ = typ.ExecAction()
	return func(ctx context.Context, command, cwd, home string, notice io.Writer) (CommandCheck, error) {
		defer p.active.hold(s.ID)()
		_, got, err := p.authorizeCommand(ctx, p.ctl.Policy(), base, commandInput{command, cwd, home}, notice)
		if err != nil {
			p.denyCommand(err, s.ID, r, got)
			return CommandCheck{}, err
		}
		return CommandCheck{Classes: got.Classes, Paths: got.Paths}, nil
	}
}

// denyCommand reports a refused console line of session id in the next
// heartbeat; like a refused Open, ErrNotReady is not reported.
func (p *Pipeline) denyCommand(err error, id uuid.UUID, r OpenRequest, req AccessRequest) {
	reason := denialReason(err)
	if reason == "" {
		return
	}
	var reasons []string
	if cd, ok := errors.AsType[*CommandDeniedError](err); ok {
		reasons = cd.Reasons
	}
	p.denials.add(newCommandDenial(reason, id, r, req, reasons))
}

// Bounds of the decision in a console denial (Denial in tunnel.proto).
const (
	maxDenialPaths   = 16
	maxDenialReasons = 16
)

// newCommandDenial is the denial of a console line in session id: the line,
// its classes, its first paths and the deciding policies, cut to the wire bounds.
func newCommandDenial(reason string, id uuid.UUID, r OpenRequest, req AccessRequest, reasons []string) *gatewayv1.Denial {
	d := newDenial(reason, req.User.ID, r.Identity.Value, r.Resource, KindTerminal, req.Command, r.ClientIP)
	d.SessionId, d.Classes = id.String(), req.Classes
	d.Paths = truncateAll(req.Paths, maxDenialPaths, 1024)
	d.Reasons = truncateAll(reasons, maxDenialReasons, 128)
	return d
}

// truncateAll is the first n values, each cut to size bytes.
func truncateAll(values []string, n, size int) []string {
	var out []string
	for _, v := range values[:min(len(values), n)] {
		out = append(out, TruncateUTF8(v, size))
	}
	return out
}

// result is the console result of s for the session end: the last line run,
// the classes and paths of every line; nil for any other session.
func (s *Session) result() *recorder.SessionResult {
	if !s.Console || s.Command == "" {
		return nil
	}
	return &recorder.SessionResult{Command: s.Command, Classes: s.Classes, Paths: s.Paths}
}
