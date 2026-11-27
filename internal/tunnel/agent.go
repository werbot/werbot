package tunnel

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// AgentRef is the agent run an agent certificate belongs to.
type AgentRef struct {
	AgentID, RunID uuid.UUID
	Limits         AgentLimits
}

// AgentLimits bound the database sessions of an agent run.
type AgentLimits struct {
	MaxRows, MaxQueries int
	MaxResultBytes      int64
	StatementTimeout    time.Duration
}

// AgentDenial is an action of an agent run a gateway refused by policy.
type AgentDenial struct {
	RunID             uuid.UUID
	Resource, Command string
	Classes, Reasons  []string
}

// Agents counts the policy denials of agent runs.
type Agents interface {
	// RecordDenial reports whether the run is paused after d; a run of another organization is apperr.ErrNotFound.
	RecordDenial(ctx context.Context, orgID uuid.UUID, d AgentDenial) (paused bool, err error)
}

// RecordDenial counts a policy denial of an agent run of the caller's organization.
func (h *Handler) RecordDenial(ctx context.Context, req *connect.Request[gatewayv1.RecordDenialRequest]) (*connect.Response[gatewayv1.RecordDenialResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	if h.d.Agents == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("agent denials are not recorded"))
	}
	m := req.Msg
	d := AgentDenial{Resource: m.GetResource(), Command: m.GetCommand(), Classes: m.GetClasses(), Reasons: m.GetReasons()}
	if d.RunID, err = parseID(m.GetRunId(), "run_id"); err != nil {
		return nil, err
	}
	paused, err := h.d.Agents.RecordDenial(ctx, g.OrgID, d)
	if err != nil {
		return nil, h.fail(ctx, "record denial", err)
	}
	return connect.NewResponse(&gatewayv1.RecordDenialResponse{Paused: paused}), nil
}

// parseAgent sets the agent and run of an agent certificate on id: both or neither.
func parseAgent(c *gatewayv1.CertIdentity, id *sshca.Identity) error {
	if c.GetAgentId() == "" && c.GetRunId() == "" {
		return nil
	}
	agent, err := parseID(c.GetAgentId(), "agent_id")
	if err != nil {
		return err
	}
	run, err := parseID(c.GetRunId(), "run_id")
	if err != nil {
		return err
	}
	id.AgentID, id.RunID = &agent, &run
	return nil
}

// agentInfo is the agent run of u for AuthenticateUser; nil for a person.
func agentInfo(u User) *gatewayv1.AgentInfo {
	if u.Agent == nil {
		return nil
	}
	l := u.Agent.Limits
	return &gatewayv1.AgentInfo{AgentId: u.Agent.AgentID.String(), RunId: u.Agent.RunID.String(), SponsorUserId: u.ID.String(),
		Limits: &gatewayv1.AgentLimits{MaxRows: int32(l.MaxRows), MaxResultBytes: l.MaxResultBytes, //nolint:gosec // agentic.Limits.Validate caps rows, queries and the timeout far below int32
			StatementTimeoutMs: int32(l.StatementTimeout.Milliseconds()), MaxQueries: int32(l.MaxQueries)}} //nolint:gosec // as above
}

// optionalID parses an id that may be absent: "" is nil.
func optionalID(s, field string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := parseID(s, field)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
