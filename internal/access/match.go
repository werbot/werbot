package access

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// breakGlassMax caps a break-glass grant.
const breakGlassMax = time.Hour

// requester is who asks: a person with teams, or an agent acting for a person.
type requester struct {
	UserID  uuid.UUID
	TeamIDs []uuid.UUID
	Agent   bool
}

// target is one resource a request covers.
type target struct {
	ID     uuid.UUID
	Labels map[string]string
}

// approver is who decides.
type approver struct {
	UserID  uuid.UUID
	TeamIDs []uuid.UUID
	Role    string
}

// matches reports whether t is picked by id, or by every label pair.
func (s Selector) matches(t target) bool {
	switch {
	case len(s.ResourceIDs) > 0:
		return slices.Contains(s.ResourceIDs, t.ID)
	case len(s.Labels) > 0:
		for k, v := range s.Labels {
			if got, ok := t.Labels[k]; !ok || got != v {
				return false
			}
		}
		return true
	}
	return false
}

// includes reports whether r is named: an agent only through AllAgents, a person through AllUsers, id or team.
func (s Subjects) includes(r requester) bool {
	if r.Agent {
		return s.AllAgents
	}
	return s.AllUsers || slices.Contains(s.Users, r.UserID) ||
		slices.ContainsFunc(r.TeamIDs, func(id uuid.UUID) bool { return slices.Contains(s.Teams, id) })
}

// matchPolicies keeps, by name, the policies that include r and cover every target; breakGlass keeps only break-glass ones.
func matchPolicies(ps []ApprovalPolicy, r requester, ts []target, breakGlass bool) []ApprovalPolicy {
	if len(ts) == 0 {
		return nil
	}
	var out []ApprovalPolicy
	for _, p := range ps {
		uncovered := slices.ContainsFunc(ts, func(t target) bool { return !p.Selector.matches(t) })
		if (breakGlass && !p.BreakGlass) || !p.Requesters.includes(r) || uncovered {
			continue
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b ApprovalPolicy) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// terms is what matching policies together demand.
type terms struct {
	Auto          bool          // every matching policy is auto
	MaxDuration   time.Duration // the smallest of the matching policies
	RequireReason bool          // any
	FourEyes      bool          // any
	Approvers     Subjects      // union over manual policies
	Admins        bool          // a manual policy has no approvers: owner and admin decide
	PolicyID      uuid.UUID     // first manual policy, else first
}

// combine merges ms (not empty, in name order): one manual policy makes the request manual.
func combine(ms []ApprovalPolicy) terms {
	t := terms{Auto: true, MaxDuration: ms[0].MaxDuration, PolicyID: ms[0].ID}
	manualSeen := false
	for _, p := range ms {
		t.MaxDuration = min(t.MaxDuration, p.MaxDuration)
		t.RequireReason = t.RequireReason || p.RequireReason
		t.FourEyes = t.FourEyes || p.FourEyes
		if p.Mode != ModeManual {
			continue
		}
		t.Auto = false
		if !manualSeen {
			t.PolicyID, manualSeen = p.ID, true
		}
		if len(p.Approvers.Users) == 0 && len(p.Approvers.Teams) == 0 {
			t.Admins = true
		}
		t.Approvers.Users = append(t.Approvers.Users, p.Approvers.Users...)
		t.Approvers.Teams = append(t.Approvers.Teams, p.Approvers.Teams...)
	}
	return t
}

// commandTerms is combine for a command: never auto; without policies owner and admin decide.
func commandTerms(ms []ApprovalPolicy) terms {
	if len(ms) == 0 {
		return terms{Admins: true}
	}
	t := combine(ms)
	t.Auto = false
	// a command is never auto-approved: with no approver named, owners and admins decide
	if len(t.Approvers.Users) == 0 && len(t.Approvers.Teams) == 0 {
		t.Admins = true
	}
	return t
}

// canDecide reports whether who may decide a. Nobody decides own access request; the sponsor of an agent command may unless four eyes.
func canDecide(a Approval, t terms, who approver) bool {
	if who.UserID == a.RequesterID && (a.AgentRunID == nil || t.FourEyes) {
		return false
	}
	if t.Admins && isManager(who.Role) {
		return true
	}
	return slices.Contains(t.Approvers.Users, who.UserID) ||
		slices.ContainsFunc(who.TeamIDs, func(id uuid.UUID) bool { return slices.Contains(t.Approvers.Teams, id) })
}

// grantDuration picks the grant length: req 0 gives defTTL (one hour for break-glass), clamped to the limit; a longer req is refused.
func grantDuration(req time.Duration, t terms, maxTTL, defTTL time.Duration, breakGlass bool) (time.Duration, error) {
	if req < 0 {
		return 0, invalid("duration must not be negative")
	}
	limit := maxTTL
	if t.MaxDuration > 0 {
		limit = min(limit, t.MaxDuration)
	}
	if breakGlass {
		limit = min(limit, breakGlassMax)
		defTTL = breakGlassMax
	}
	if req == 0 {
		return min(defTTL, limit), nil
	}
	if req > limit {
		return 0, invalid("duration exceeds %s", limit)
	}
	return req, nil
}
