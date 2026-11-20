package access

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/principal"
)

// member is a member of the organization as an approval sees it.
type member struct {
	approver
	Email, Login string
}

func (m member) manager() bool { return m.Role == principal.RoleOwner || m.Role == principal.RoleAdmin }

// evaluator holds what deciding on requests of one organization reads: its
// policies, resources with labels and members, loaded once per use case.
// ponytail: whole-organization reads; filter in SQL if organizations grow to many thousands of resources or members.
type evaluator struct {
	policies []ApprovalPolicy
	targets  []target
	names    map[uuid.UUID]string
	members  map[uuid.UUID]member
}

func loadEvaluator(ctx context.Context, st Store, orgID uuid.UUID) (evaluator, error) {
	e := evaluator{names: map[uuid.UUID]string{}, members: map[uuid.UUID]member{}}
	prows, err := st.ListApprovalPolicies(ctx, orgID)
	if err != nil {
		return e, err
	}
	for _, r := range prows {
		p, err := toPolicy(r)
		if err != nil {
			return e, err
		}
		e.policies = append(e.policies, p)
	}
	trows, err := st.ListTargets(ctx, orgID)
	if err != nil {
		return e, err
	}
	for _, r := range trows {
		t := target{ID: r.ID}
		if err := json.Unmarshal(r.Labels, &t.Labels); err != nil {
			return e, fmt.Errorf("access: labels of %s: %w", r.ID, err)
		}
		e.targets = append(e.targets, t)
		e.names[r.ID] = r.Name
	}
	mrows, err := st.ListMembers(ctx, orgID)
	if err != nil {
		return e, err
	}
	for _, r := range mrows {
		e.members[r.UserID] = member{approver{UserID: r.UserID, TeamIDs: r.TeamIds, Role: r.Role}, r.Email, r.Login}
	}
	return e, nil
}

// targetsOf returns the current resources of a: its resource, or every resource its selector matches.
func (e evaluator) targetsOf(resourceID *uuid.UUID, selector map[string]string) []target {
	sel := Selector{Labels: selector}
	if resourceID != nil {
		sel = Selector{ResourceIDs: []uuid.UUID{*resourceID}}
	}
	var out []target
	for _, t := range e.targets {
		if sel.matches(t) {
			out = append(out, t)
		}
	}
	return out
}

// requester is who asked for a: unknown users are no one.
func (e evaluator) requester(a Approval) (requester, bool) {
	m, ok := e.members[a.RequesterID]
	return requester{UserID: m.UserID, TeamIDs: m.TeamIDs, Agent: a.AgentRunID != nil}, ok
}

// terms are what the current policies demand of a; false when none matches an access request.
func (e evaluator) terms(a Approval) (terms, bool) {
	r, ok := e.requester(a)
	if !ok {
		return terms{}, false
	}
	ms := matchPolicies(e.policies, r, e.targetsOf(a.ResourceID, a.Selector), a.BreakGlass)
	if a.Kind == KindCommand {
		return commandTerms(ms), true
	}
	if len(ms) == 0 {
		return terms{}, false
	}
	return combine(ms), true
}

// canDecide reports whether user may decide a under the current policies.
func (e evaluator) canDecide(a Approval, user uuid.UUID) bool {
	who, ok := e.members[user]
	t, matched := e.terms(a)
	return ok && matched && canDecide(a, t, who.approver)
}

// visible reports whether user sees a: the requester, a possible approver, an owner or an admin.
func (e evaluator) visible(a Approval, user uuid.UUID) bool {
	return user == a.RequesterID || e.members[user].manager() || e.canDecide(a, user)
}

// membersWhere returns the members keep accepts, by user id.
func (e evaluator) membersWhere(keep func(member) bool) []member {
	var out []member
	for _, m := range e.members {
		if keep(m) {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(x, y member) int { return slices.Compare(x.UserID[:], y.UserID[:]) })
	return out
}

// approvers are the members who may decide a under t.
func (e evaluator) approvers(a Approval, t terms) []member {
	return e.membersWhere(func(m member) bool { return canDecide(a, t, m.approver) })
}
