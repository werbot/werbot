package policy

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// SnapshotAgent is an agent as the policy sees it: the Cedar entity
// Werbot::Agent::"<ID>" {sponsor: Werbot::User::"<SponsorID>", status}.
type SnapshotAgent struct {
	ID        uuid.UUID `json:"id"`
	SponsorID uuid.UUID `json:"sponsor_id"`
	Status    string    `json:"status"` // active | frozen
}

// AgentSource lists the agents of an organization and its blocked runs
// (paused, or ended within the last hour).
type AgentSource interface {
	PolicyAgents(ctx context.Context, orgID uuid.UUID) ([]SnapshotAgent, []uuid.UUID, error)
}

func userUID(id uuid.UUID) cedar.EntityUID {
	return cedar.NewEntityUID("Werbot::User", cedar.String(id.String()))
}

func agentUID(id uuid.UUID) cedar.EntityUID {
	return cedar.NewEntityUID("Werbot::Agent", cedar.String(id.String()))
}

// idString is id as a context string; nil (no run) is "", which no run id matches.
func idString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// authorizeAgent decides r of agent r.AgentID twice: as its sponsor r.UserID
// (with the sponsor's teams) and as the agent, and allows only when both
// passes allow. An agent missing from the snapshot or sponsored by another
// user denies (fail-closed).
// The gateway PDP mirrors this in internal/gateway/agent.go: change both together.
func (ps *PolicySet) authorizeAgent(r Request) Decision {
	agent := agentUID(*r.AgentID)
	e, ok := ps.entities[agent]
	sponsor, _ := e.Attributes.Get("sponsor")
	if !ok || sponsor != userUID(r.UserID) {
		return Decision{Reasons: []string{"agent:missing"}}
	}
	return mergeAgent(ps.decidePass(r, userUID(r.UserID)), ps.decidePass(r, agent))
}

// mergeAgent combines the sponsor and agent passes: allow only when both
// allow; reasons are the union, sorted; a deny carries obligations only when
// every denying pass carries them (their union), and the message of the
// first denying pass that has one.
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

// subject is the user of a request, or the agent whose sponsor it is.
type subject struct {
	user  uuid.UUID
	agent *uuid.UUID
}

// sponsor is the user of s: its sponsor in snap for an agent; ErrNotFound
// when snap has no such agent.
func (s subject) sponsor(snap Snapshot) (uuid.UUID, error) {
	if s.agent == nil {
		return s.user, nil
	}
	return agentSponsor(snap, *s.agent)
}

// agentSponsor is the sponsor of agentID in snap; ErrNotFound when the
// snapshot has no such agent.
func agentSponsor(snap Snapshot, agentID uuid.UUID) (uuid.UUID, error) {
	i := slices.IndexFunc(snap.Agents, func(a SnapshotAgent) bool { return a.ID == agentID })
	if i < 0 {
		return uuid.Nil, fmt.Errorf("agent not found: %w", apperr.ErrNotFound)
	}
	return snap.Agents[i].SponsorID, nil
}
