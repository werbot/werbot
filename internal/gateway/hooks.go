package gateway

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/gateway/mask"
)

// AgentLimits bound the database work of one agent session (agent.limits).
type AgentLimits struct {
	MaxRows          int
	MaxResultBytes   int64
	StatementTimeout time.Duration
	MaxQueries       int
}

// ApprovalInput is an operation the approvers read: its exact text, its
// classes and the policies that asked for the approval.
type ApprovalInput struct {
	Command          string
	Classes, Reasons []string
}

// ApprovalTicket is an approval request; Status is "pending" or "approved"
// (an approval granted before).
type ApprovalTicket struct{ ID, URL, Status string }

// Approver requests an approval for the user, resource and agent run of a session.
type Approver interface {
	RequestApproval(ctx context.Context, command string, classes, reasons []string) (ApprovalTicket, error)
}

// SessionHooks are what a protocol connector needs from the pipeline for one
// session. The zero value checks no operation (stage 17 behaviour).
type SessionHooks struct {
	// PDP decides the session's operations on the current snapshot for its
	// user, resource and address: a request sets only its operation fields.
	PDP PDP
	// Approve requests an approval and waits for it: nil once approved;
	// ErrApprovalDenied, ErrApprovalTimeout, ErrNotReady or the context
	// error. notify gets the ticket before any wait; timeout 0 marks an
	// approval granted before, with no wait.
	Approve func(ctx context.Context, in ApprovalInput, notify func(id, url string, timeout time.Duration) error) error
	// Approver requests an approval without waiting (retry mode).
	Approver Approver
	// RecordDenial reports a denied operation: an agent's at once through a
	// queue, a person's in the next heartbeat.
	RecordDenial func(in ApprovalInput)
	// WriteCredential reveals the primary credential for an approved write.
	WriteCredential func(ctx context.Context, approvalID string, opSHA [32]byte) (Secret, error)
	// Limits are the agent's limits; nil for a person.
	Limits *AgentLimits
	// Mask masks the session credential and credential-shaped text in a
	// value sent to the client; nil when the output is not masked.
	Mask func(p []byte) []byte
}

type hooksKey struct{}

// WithHooks is ctx carrying h.
func WithHooks(ctx context.Context, h SessionHooks) context.Context {
	return context.WithValue(ctx, hooksKey{}, h)
}

// HooksFrom is the hooks of ctx; the zero value when none are set.
func HooksFrom(ctx context.Context) SessionHooks {
	h, _ := ctx.Value(hooksKey{}).(SessionHooks)
	return h
}

// hooks are the session hooks of s, bound to the request Open decided.
func (p *Pipeline) hooks(s *Session) SessionHooks {
	h := SessionHooks{
		PDP: sessionPDP{policy: p.ctl.Policy, base: connectOf(p.active.request(s.ID))},
		Approve: func(ctx context.Context, in ApprovalInput, notify func(id, url string, timeout time.Duration) error) error {
			return p.approveQuery(ctx, s, in, notify)
		},
		Approver:     sessionApprover{p: p, s: s},
		RecordDenial: p.recordDenial(s),
		WriteCredential: func(ctx context.Context, approvalID string, opSHA [32]byte) (Secret, error) {
			return p.ctl.GetWriteCredential(ctx, s.ID, approvalID, opSHA)
		},
	}
	if a := s.User.Agent; a != nil {
		h.Limits = &a.Limits
	}
	if maskEnabled(p.ctl.Policy().MaskOutput(), s.User.Agent != nil) {
		secrets := [][]byte{s.Secret.Value}
		h.Mask = func(b []byte) []byte { return mask.Bytes(b, secrets, true) }
	}
	return h
}

// request is the request Open decided for session id; zero when unknown.
func (a *activeSessions) request(id uuid.UUID) AccessRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t := a.sessions[id]; t != nil {
		return t.req
	}
	return AccessRequest{}
}

// sessionPDP decides operation requests with the session's user, resource,
// address and kind on the snapshot policy returns now.
type sessionPDP struct {
	policy func() *Policy
	base   AccessRequest
}

func (s sessionPDP) Decide(r AccessRequest) Decision { return s.policy().Decide(s.with(r)) }

func (s sessionPDP) Version() string { return s.policy().Version() }

func (s sessionPDP) QueryProfile(r AccessRequest) QueryProfile {
	return s.policy().QueryProfile(s.with(r))
}

// with is the base request with the operation fields of r.
func (s sessionPDP) with(r AccessRequest) AccessRequest {
	b := s.base
	b.Action, b.Command, b.Classes, b.ApprovalGranted = r.Action, r.Command, r.Classes, r.ApprovalGranted
	b.DBName, b.Tables, b.WriteTables = r.DBName, r.Tables, r.WriteTables
	return b
}

// approveQuery requests an approval of an operation of s and waits for it,
// holding the idle limit meanwhile. An unreachable server is ErrNotReady, a
// refused request ErrApprovalDenied.
func (p *Pipeline) approveQuery(ctx context.Context, s *Session, in ApprovalInput, notify func(id, url string, timeout time.Duration) error) error {
	defer p.active.hold(s.ID)()
	t, err := p.ctl.RequestApproval(ctx, queryApproval(s, in))
	switch {
	case unreachable(err):
		return controlErr(err)
	case err != nil:
		p.log.Warn("approval: request refused", "session_id", s.ID, "error", err)
		return ErrApprovalDenied
	case t.Status == "approved":
		return notify(t.ID, t.URL, 0)
	}
	if err := notify(t.ID, t.URL, max(t.ExpiresAt.Sub(p.now()), time.Second)); err != nil {
		return err
	}
	return p.awaitApproval(ctx, t)
}

// queryApproval asks for in on the user, resource and agent run of s.
func queryApproval(s *Session, in ApprovalInput) approvalRequest {
	return approvalRequest{UserID: s.User.ID, ResourceID: s.Resource.ID, AgentRunID: s.User.runID(),
		Command: in.Command, Classes: in.Classes, Reasons: in.Reasons}
}

// sessionApprover requests approvals for session s without waiting.
type sessionApprover struct {
	p *Pipeline
	s *Session
}

func (a sessionApprover) RequestApproval(ctx context.Context, command string, classes, reasons []string) (ApprovalTicket, error) {
	t, err := a.p.ctl.RequestApproval(ctx, queryApproval(a.s, ApprovalInput{Command: command, Classes: classes, Reasons: reasons}))
	return ApprovalTicket{ID: t.ID, URL: t.URL, Status: t.Status}, err
}
