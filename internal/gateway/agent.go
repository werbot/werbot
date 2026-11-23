package gateway

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"
)

// EnvToolCall is the SSH environment variable an agent's client sets to the
// tool call (a uuid) its exec belongs to.
const EnvToolCall = "WERBOT_TOOL_CALL"

// Refusals of an agent's session by its run, not by policy: the front reads
// "Agent limit exceeded" and "Agent run is not active".
var (
	ErrAgentLimit        = errors.New("agent limit exceeded")
	ErrAgentRunNotActive = errors.New("agent run is not active")
)

// agentRunText is the user text of a refusal of an agent's run; "" for any other error.
func agentRunText(err error) string {
	switch {
	case errors.Is(err, ErrAgentLimit):
		return "Agent limit exceeded"
	case errors.Is(err, ErrAgentRunNotActive):
		return "Agent run is not active"
	}
	return ""
}

// agentSnapshotGrace is how long after AuthenticateUser an agent missing
// from the snapshot is decided as an active agent of its sponsor: the server
// checked the agent then, and the snapshot that lists it may still be on its
// way. Later a missing agent denies.
const agentSnapshotGrace = 30 * time.Second

// Agent is the agent run of a user certificate; the user is its sponsor.
// AuthAt is when AuthenticateUser accepted it.
type Agent struct {
	ID, RunID uuid.UUID
	AuthAt    time.Time
}

// AuthorizeAgent decides r of the agent r.User.Agent twice: as its sponsor
// r.User and as the agent, and allows only when both passes allow. Decide
// calls it after its request checks. An agent sponsored by another user, or
// one missing from the snapshot past agentSnapshotGrace, denies (fail-closed).
// It mirrors policy.(*PolicySet).authorizeAgent in internal/policy/agents.go:
// change both together; TestPolicyParityAgents keeps them equal.
func (p *Policy) AuthorizeAgent(r AccessRequest) Decision {
	agent, ok := p.agentEntity(r)
	if !ok {
		return Decision{Reasons: []string{"agent:missing"}}
	}
	return mergeAgent(p.decidePass(r, nil), p.decidePass(r, &agent))
}

// agentEntity is the entity of r.User.Agent: the snapshot's when it lists the
// agent, which must be sponsored by r.User; an active agent of r.User while
// the snapshot may not list it yet; false otherwise.
func (p *Policy) agentEntity(r AccessRequest) (cedar.Entity, bool) {
	a := r.User.Agent
	uid := cedar.NewEntityUID("Werbot::Agent", cedar.String(a.ID.String()))
	sponsor := cedar.NewEntityUID("Werbot::User", cedar.String(r.User.ID.String()))
	if e, ok := p.entities[uid]; ok {
		s, _ := e.Attributes.Get("sponsor")
		return e, s == sponsor
	}
	if r.at().Sub(a.AuthAt) >= agentSnapshotGrace {
		return cedar.Entity{}, false
	}
	return cedar.Entity{UID: uid, Attributes: cedar.NewRecord(cedar.RecordMap{"sponsor": sponsor, "status": cedar.String("active")})}, true
}

// mergeAgent combines the sponsor and agent passes: allow only when both
// allow; reasons are the union, sorted; a deny carries obligations only when
// every denying pass carries them (their union), and the message of the
// first denying pass that has one. It mirrors policy.mergeAgent: change both together.
func mergeAgent(user, agent Decision) Decision {
	out := Decision{Allow: user.Allow && agent.Allow,
		Reasons: slices.Compact(slices.Sorted(slices.Values(slices.Concat(user.Reasons, agent.Reasons))))}
	if out.Allow {
		return out
	}
	var obligations []string
	every := true
	for _, d := range []Decision{user, agent} {
		if d.Allow {
			continue
		}
		out.Message = cmp.Or(out.Message, d.Message)
		every = every && len(d.Obligations) > 0
		obligations = append(obligations, d.Obligations...)
	}
	if every {
		out.Obligations = slices.Compact(slices.Sorted(slices.Values(obligations)))
	}
	return out
}

// runID is the agent run of u; uuid.Nil for a person.
func (u User) runID() uuid.UUID {
	if u.Agent == nil {
		return uuid.Nil
	}
	return u.Agent.RunID
}

// optionalID is id on the wire; nil (none) is "".
func optionalID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// idString is id on the wire and in a Cedar context: uuid.Nil (none) is "",
// which no id matches.
func idString(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
