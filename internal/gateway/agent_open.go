package gateway

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/sshca"
)

// startArgs is what StartSession records besides the access request: the
// front and transport of the session and, for an agent's, its run and the
// tool call it serves (zero for a person's session).
type startArgs struct {
	Front           string
	Transport       string // "tcp" | "ws" | "internal"
	RunID, ToolCall uuid.UUID
}

// startOf is the start of u's session for r: the front and transport of r
// (none is tcp) and, for an agent, the run of u's agent and the tool call
// of r; a person's tool call is ignored.
func startOf(u User, r OpenRequest) startArgs {
	a := startArgs{Front: r.Front, Transport: cmp.Or(r.Transport, "tcp")}
	if u.Agent != nil {
		a.RunID, a.ToolCall = u.Agent.RunID, r.ToolCall
	}
	return a
}

// toolCallOf is the tool call of an SSH session environment (sshca.EnvToolCall);
// uuid.Nil when it is unset or not a uuid.
func toolCallOf(env []string) uuid.UUID {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, sshca.EnvToolCall+"="); ok {
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

// stateReasons are the policies that deny an agent for its run's own state;
// a lock ("lock:<id>") is one too.
var stateReasons = map[string]bool{"agent:baseline:blocked_runs": true, "agent:baseline:frozen": true, "agent:missing": true}

// stateDenial reports a denial decided by the run's state: a blocked run, a
// frozen or missing agent or a lock forbids every action of the run, so the
// denial is audited but never counted toward the pause, as a refused run is
// not. Any such reason decides it: the reasons also hold the permits of an
// allowing pass (the sponsor's binding), and a run its state forbids cannot
// act whatever else it tried.
func stateDenial(reasons []string) bool {
	return slices.ContainsFunc(reasons, func(r string) bool { return stateReasons[r] || strings.HasPrefix(r, "lock:") })
}

// reportAgentDenial records a policy denial of an agent's request req at
// once through RecordDenial; a denial by the run's state, or one RecordDenial
// failed on, goes to the next heartbeat with the run. It reports whether err
// was such a denial.
func (p *Pipeline) reportAgentDenial(ctx context.Context, r OpenRequest, req AccessRequest, err error) bool {
	a := req.User.Agent
	if a == nil || !agentPolicyDenial(err) {
		return false
	}
	target := cmp.Or(r.Command, r.Target)
	reasons := truncateAll(deniedReasons(err), maxDenialReasons, 128)
	if stateDenial(reasons) {
		p.denyAgentLater(r, req, err, target, reasons)
		return true
	}
	paused, rerr := p.ctl.RecordDenial(context.WithoutCancel(ctx), a.RunID, TruncateUTF8(r.Resource, 63), TruncateUTF8(target, 1024), req.Classes, reasons)
	if rerr != nil {
		p.log.Warn("agent: denial not recorded, reported in the next heartbeat", "run_id", a.RunID, "error", rerr)
		p.denyAgentLater(r, req, err, target, reasons)
		return true
	}
	if paused { // ponytail: paused feeds only this log; the pause reaches the PDP via BlockedRuns. Drop it at the next proto break.
		p.log.Info("agent: run paused after a denial", "run_id", a.RunID)
	}
	return true
}

// denyAgentLater reports an agent's denial in the next heartbeat with its run,
// where the server audits it without counting it.
func (p *Pipeline) denyAgentLater(r OpenRequest, req AccessRequest, err error, target string, reasons []string) {
	d := newDenial(denialReason(err), req.User.ID, r.Identity.fingerprint(), r.Resource, r.Kind, target, r.ClientIP)
	d.AgentRunId, d.Classes, d.Reasons = req.User.Agent.RunID.String(), req.Classes, reasons
	p.denials.add(d)
}
