package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/shellclass"
)

// Errors of a command the approvers refused or did not decide in time.
var (
	ErrApprovalDenied  = errors.New("approval denied")
	ErrApprovalTimeout = errors.New("Approval timed out") //nolint:staticcheck // the user reads this text
)

// CommandDeniedError is a refused command; Err is ErrDenied or ErrApprovalDenied,
// Message the @message of the deciding rule and Reasons the ids of the
// policies that decided it.
type CommandDeniedError struct {
	Err     error
	Message string
	Reasons []string
}

// Error is the text the user reads: "Command denied: <Message>"; without a
// message a refused approval reads "Command denied: approval denied" and a
// plain deny "Access denied", as a refused connection does.
func (e *CommandDeniedError) Error() string {
	switch {
	case e.Message != "":
		return "Command denied: " + e.Message
	case errors.Is(e.Err, ErrApprovalDenied):
		return "Command denied: approval denied"
	}
	return "Access denied"
}

func (e *CommandDeniedError) Unwrap() error { return e.Err }

// withExecAction sets the exec action of typ on req when r runs a command: an
// exec, or a terminal with a command. A type without one runs no commands, and
// req stays a connect request.
func withExecAction(req AccessRequest, r OpenRequest, typ connector.Type) AccessRequest {
	if r.Kind == KindExec || (r.Kind == KindTerminal && r.Command != "") {
		req.Action, _ = typ.ExecAction()
	}
	return req
}

// commandInput is a command to check: Text runs in Cwd ("~" for an exec);
// Home, when set, resolves "~" in the paths and the path rules (a console line).
type commandInput struct{ Text, Cwd, Home string }

// script is the text the approvers read: a console line with the cd to its cwd
// it runs after (ConsoleScript), an exec's command as it is.
func (in commandInput) script() string {
	if in.Home == "" {
		return in.Text
	}
	return ConsoleScript(in.Cwd, in.Text)
}

// ConsoleScript is what the target runs for a console line: the line only in
// the checked cwd, or not at all.
func ConsoleScript(cwd, line string) string {
	return "cd -- '" + strings.ReplaceAll(cwd, "'", `'\''`) + "' || exit 1\n" + line
}

// withCommand returns req with the command of in analysed on p: its classes
// and paths, and the path rules they hit or leave. The command is classified
// with the snapshot class rules and the host of the resource.
func (p *Policy) withCommand(req AccessRequest, in commandInput) AccessRequest {
	ep, _, _ := req.Resource.Endpoint() // an invalid config leaves no host: every remote host is egress
	a, rules := p.Analyze(in.Text, ep.Host, in.Cwd), p.PathRules()
	if in.Home != "" {
		a, rules = a.WithHome(in.Home), shellclass.ExpandHome(rules, in.Home)
	}
	req.Command, req.Classes, req.Paths = in.Text, nil, nil
	for _, c := range a.Classes {
		req.Classes = append(req.Classes, string(c))
	}
	for _, r := range a.Paths {
		req.Paths = append(req.Paths, r.String())
	}
	req.PathHits, req.PathOutside = shellclass.EvalPaths(rules, a.Paths, a.Unresolved)
	return req
}

// authorizeCommand decides the command of in on req.Action; a request without
// an action passes unchanged. A deny that only an approval lifts waits for the
// approvers, telling the user on notice, and is then decided again. It returns
// the decided request and the snapshot it was decided on: the session runs on
// that request's user and resource, and the sweeper re-decides it.
func (p *Pipeline) authorizeCommand(ctx context.Context, pol *Policy, req AccessRequest, in commandInput, notice io.Writer) (*Policy, AccessRequest, error) {
	if req.Action == "" {
		return pol, req, nil
	}
	req = pol.withCommand(req, in)
	d := pol.Decide(req)
	switch {
	case d.Allow:
		return pol, req, nil
	case !slices.Equal(d.Obligations, []string{ObligationApproval}):
		return pol, req, &CommandDeniedError{Err: ErrDenied, Message: d.Message, Reasons: d.Reasons}
	}
	if err := p.approve(ctx, req, d, in.script(), notice); err != nil {
		return pol, req, err
	}
	return p.recheckApproved(req, in)
}

// recheckApproved analyses and decides an approved command again on the
// current snapshot at the current time, since the wait may have changed the
// resource, the membership, connect or the command and path rules: a deny
// refuses it, with the message of the new command decision. A resource whose
// type or connector config (its endpoint included) changed is refused too: the
// approvers saw the old one.
func (p *Pipeline) recheckApproved(req AccessRequest, in commandInput) (*Policy, AccessRequest, error) {
	pol := p.ctl.Policy()
	cur, ok := recheck(pol, evaluation{req: connectOf(req)}, p.now())
	if !ok || cur.Resource.Type != req.Resource.Type || !bytes.Equal(cur.Resource.Config, req.Resource.Config) {
		return pol, req, &CommandDeniedError{Err: ErrDenied}
	}
	cur.Action, cur.ApprovalGranted = req.Action, true
	cur = pol.withCommand(cur, in)
	if d := pol.Decide(cur); !d.Allow {
		return pol, cur, &CommandDeniedError{Err: ErrDenied, Message: d.Message, Reasons: d.Reasons}
	}
	return pol, cur, nil
}

// connectOf is the connect request of a session's request.
func connectOf(req AccessRequest) AccessRequest {
	req.Action, req.Command, req.Classes, req.ApprovalGranted, req.Console = "", "", nil, false, false
	req.Paths, req.PathHits, req.PathOutside = nil, nil, nil
	return req
}

// commandAllowed re-decides the action of a session of kind. A shell passes
// only on a plain allow (a raw shell under a new console rule ends); so does
// the shell decision of a terminal with a command (Ruling O1). A command is
// analysed again from "~", for the current class and path rules, and decided
// with its approval; only a deny that no obligation lifts refuses it. A
// session without an action passes.
func commandAllowed(pol *Policy, req AccessRequest, kind Kind) bool {
	typ, _ := connector.Lookup(req.Resource.Type)
	shell, hasShell := typ.ShellAction()
	switch req.Action {
	case "":
		return true
	case shell:
		return pol.Decide(req).Allow
	}
	if kind == KindTerminal && hasShell && !shellAllowed(pol, req, shell) {
		return false
	}
	d := pol.Decide(pol.withCommand(req, commandInput{Text: req.Command, Cwd: "~"}))
	return d.Allow || len(d.Obligations) > 0
}

// shellAllowed is the plain allow of action shell for the user and resource of req.
func shellAllowed(pol *Policy, req AccessRequest, shell string) bool {
	s := connectOf(req)
	s.Action = shell
	return pol.Decide(s).Allow
}

// CommandErrorText is the text the user reads for a refused command or a
// failed check; anything else reads "Access denied".
func CommandErrorText(err error) string {
	switch {
	case errors.Is(err, ErrNotReady):
		return "Gateway is not ready, try again later"
	case errors.Is(err, ErrApprovalTimeout):
		return ErrApprovalTimeout.Error()
	}
	if cd, ok := errors.AsType[*CommandDeniedError](err); ok {
		return cd.Error()
	}
	return "Access denied"
}

// controlErr maps a control plane error: an outage is ErrNotReady, anything else ErrDenied.
func controlErr(err error) error {
	if unreachable(err) {
		return fmt.Errorf("%w: %w", ErrNotReady, err)
	}
	return fmt.Errorf("%w: %w", ErrDenied, err)
}
