package gateway

import (
	"cmp"
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// agentStart is the agent run of a session and the tool call it serves; zero
// for a person's session.
type agentStart struct{ RunID, ToolCall uuid.UUID }

// startOf is the agent start of u's session for r: the run of u's agent and
// the tool call of r; zero for a person, whose tool call is ignored.
func startOf(u User, r OpenRequest) agentStart {
	if u.Agent == nil {
		return agentStart{}
	}
	return agentStart{RunID: u.Agent.RunID, ToolCall: r.ToolCall}
}

// toolCallOf is the tool call of an SSH session environment (EnvToolCall);
// uuid.Nil when it is unset or not a uuid.
func toolCallOf(env []string) uuid.UUID {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, EnvToolCall+"="); ok {
			id, err := uuid.Parse(v)
			if err != nil {
				return uuid.Nil
			}
			return id
		}
	}
	return uuid.Nil
}

// startErr maps a StartSession error: the server refusing an agent's run is
// ErrAgentLimit or ErrAgentRunNotActive; anything else is controlErr.
func startErr(err error) error {
	if ce, ok := errors.AsType[*connect.Error](err); ok && ce.Code() == connect.CodeFailedPrecondition {
		switch ce.Message() {
		case ErrAgentLimit.Error():
			return ErrAgentLimit
		case ErrAgentRunNotActive.Error():
			return ErrAgentRunNotActive
		}
	}
	return controlErr(err)
}

// accessDenied is a connect request the PDP denied, with the policies that
// decided it; it reads like ErrDenied.
type accessDenied struct{ reasons []string }

func (accessDenied) Error() string { return ErrDenied.Error() }
func (accessDenied) Unwrap() error { return ErrDenied }

// deniedReasons are the policies that decided a refused request; nil when
// err carries none.
func deniedReasons(err error) []string {
	if cd, ok := errors.AsType[*CommandDeniedError](err); ok {
		return cd.Reasons
	}
	if d, ok := errors.AsType[accessDenied](err); ok {
		return d.reasons
	}
	return nil
}

// agentPolicyDenial reports whether an Open error refused an agent by policy:
// a run counts these toward its pause. Refusals of the run itself, an
// unready or busy gateway are not.
func agentPolicyDenial(err error) bool {
	return errors.Is(err, ErrDenied) || errors.Is(err, ErrShellDenied) || errors.Is(err, ErrTargetNotAllowed) ||
		errors.Is(err, ErrApprovalDenied) || errors.Is(err, ErrApprovalTimeout)
}

// reportAgentDenial records a policy denial of an agent's request req at
// once through RecordDenial; when that fails the denial goes to the next
// heartbeat with the run. It reports whether err was such a denial.
func (p *Pipeline) reportAgentDenial(ctx context.Context, r OpenRequest, req AccessRequest, err error) bool {
	a := req.User.Agent
	if a == nil || !agentPolicyDenial(err) {
		return false
	}
	target := cmp.Or(r.Command, r.Target)
	reasons := truncateAll(deniedReasons(err), maxDenialReasons, 128)
	paused, rerr := p.ctl.RecordDenial(context.WithoutCancel(ctx), a.RunID, TruncateUTF8(r.Resource, 63), TruncateUTF8(target, 1024), req.Classes, reasons)
	if rerr != nil {
		p.log.Warn("agent: denial not recorded, reported in the next heartbeat", "run_id", a.RunID, "error", rerr)
		d := newDenial(denialReason(err), req.User.ID, r.Identity.Value, r.Resource, r.Kind, target, r.ClientIP)
		d.AgentRunId, d.Classes, d.Reasons = a.RunID.String(), req.Classes, reasons
		p.denials.add(d)
		return true
	}
	if paused {
		p.log.Info("agent: run paused after a denial", "run_id", a.RunID)
	}
	return true
}
