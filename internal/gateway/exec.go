package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"

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
// the resource host. It returns the decided request: the sweeper re-decides it.
func (p *Pipeline) authorizeCommand(_ context.Context, pol *Policy, req AccessRequest, command string, _ io.Writer) (AccessRequest, error) {
	if req.Action == "" {
		return req, nil
	}
	ep, _, _ := req.Resource.Endpoint() // an invalid config leaves no host: every remote host is egress
	req.Command, req.Classes = command, pol.Classify(command, ep.Host)
	if d := pol.Decide(req); !d.Allow {
		return req, &CommandDeniedError{Err: ErrDenied, Message: d.Message}
	}
	return req, nil
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
