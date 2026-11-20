package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/werbot/werbot/internal/connector"
)

// Errors of a command the approvers refused or did not decide in time.
var (
	ErrApprovalDenied  = errors.New("approval denied")
	ErrApprovalTimeout = errors.New("Approval timed out") //nolint:staticcheck // the user reads this text
)

// CommandDeniedError is a refused command; Err is ErrDenied or ErrApprovalDenied
// and Message the @message of the deciding rule.
type CommandDeniedError struct {
	Err     error
	Message string
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

// authorizeCommand decides command on req.Action; a request without an action
// passes unchanged. The command is classified with the snapshot class rules and
// the resource host. A deny that only an approval lifts waits for the
// approvers, telling the user on notice, and is then decided again. It returns
// the decided request and the snapshot it was decided on: the session runs on
// that request's user and resource, and the sweeper re-decides it.
func (p *Pipeline) authorizeCommand(ctx context.Context, pol *Policy, req AccessRequest, command string, notice io.Writer) (*Policy, AccessRequest, error) {
	if req.Action == "" {
		return pol, req, nil
	}
	ep, _, _ := req.Resource.Endpoint() // an invalid config leaves no host: every remote host is egress
	req.Command, req.Classes = command, pol.Classify(command, ep.Host)
	d := pol.Decide(req)
	switch {
	case d.Allow:
		return pol, req, nil
	case !slices.Equal(d.Obligations, []string{ObligationApproval}):
		return pol, req, &CommandDeniedError{Err: ErrDenied, Message: d.Message}
	}
	if err := p.approve(ctx, req, d, notice); err != nil {
		return pol, req, err
	}
	return p.recheckApproved(req)
}

// recheckApproved decides an approved command again on the current snapshot at
// the current time, since the wait may have changed the resource, the
// membership, connect or the command rules: a deny refuses it, with the
// message of the new command decision. A resource whose type or connector
// config (its endpoint included) changed is refused too: the approvers saw the
// old one.
func (p *Pipeline) recheckApproved(req AccessRequest) (*Policy, AccessRequest, error) {
	pol := p.ctl.Policy()
	cur, ok := recheck(pol, evaluation{req: connectOf(req)}, p.now())
	if !ok || cur.Resource.Type != req.Resource.Type || !bytes.Equal(cur.Resource.Config, req.Resource.Config) {
		return pol, req, &CommandDeniedError{Err: ErrDenied}
	}
	cur.Action, cur.Command, cur.Classes, cur.ApprovalGranted = req.Action, req.Command, req.Classes, true
	if d := pol.Decide(cur); !d.Allow {
		return pol, cur, &CommandDeniedError{Err: ErrDenied, Message: d.Message}
	}
	return pol, cur, nil
}

// connectOf is the connect request of a session's request.
func connectOf(req AccessRequest) AccessRequest {
	req.Action, req.Command, req.Classes, req.ApprovalGranted = "", "", nil, false
	return req
}

// commandAllowed re-decides the command of a session with its classes and
// approval; only a deny that no obligation lifts refuses it. A session without
// a command passes.
func commandAllowed(pol *Policy, req AccessRequest) bool {
	if req.Action == "" {
		return true
	}
	d := pol.Decide(req)
	return d.Allow || len(d.Obligations) > 0
}

// controlErr maps a control plane error: an outage is ErrNotReady, anything else ErrDenied.
func controlErr(err error) error {
	if unreachable(err) {
		return fmt.Errorf("%w: %w", ErrNotReady, err)
	}
	return fmt.Errorf("%w: %w", ErrDenied, err)
}
