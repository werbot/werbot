package access_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func (e env) policy(t *testing.T) access.ApprovalPolicy {
	t.Helper()
	return access.ApprovalPolicy{Name: "prod", Mode: access.ModeManual,
		Selector:   access.Selector{Labels: map[string]string{"env": "prod"}},
		Requesters: access.Subjects{AllUsers: true}, RequireReason: true}
}

func (e env) auditCount(t *testing.T, action string, id uuid.UUID) (n int) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(e.ctx,
		`SELECT count(*) FROM audit_event WHERE org_id = $1 AND action = $2 AND target_type = 'approval_policy' AND target_id = $3`,
		e.org, action, id.String()).Scan(&n))
	return n
}

func TestApprovalPolicyCRUD(t *testing.T) {
	e := setup(t)
	p := e.principal()
	b := e.policy(t)
	b.Name = "b"
	a := e.policy(t)
	a.Name = "a"
	a.BreakGlass = true
	got, err := e.svc.CreateApprovalPolicy(e.ctx, p, b)
	require.NoError(t, err)
	assert.Equal(t, 8*time.Hour, got.MaxDuration, "unset max_duration defaults to 8h")
	assert.Equal(t, &e.owner, got.CreatedBy)
	assert.Equal(t, e.org, got.OrgID)
	created, err := e.svc.CreateApprovalPolicy(e.ctx, p, a)
	require.NoError(t, err)

	list, err := e.svc.ListApprovalPolicies(e.ctx, e.org)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, []string{"a", "b"}, []string{list[0].Name, list[1].Name}, "ordered by name")

	in := got
	in.Mode, in.MaxDuration = access.ModeAuto, 2*time.Hour
	upd, err := e.svc.UpdateApprovalPolicy(e.ctx, p, in)
	require.NoError(t, err)
	assert.Equal(t, access.ModeAuto, upd.Mode)
	assert.Equal(t, 2*time.Hour, upd.MaxDuration)
	assert.Equal(t, got.CreatedAt, upd.CreatedAt)

	_, err = e.svc.UpdateApprovalPolicy(e.ctx, p, access.ApprovalPolicy{ID: uuid.New(), Name: "x", Mode: access.ModeAuto,
		Selector: b.Selector, Requesters: b.Requesters})
	assert.ErrorIs(t, err, apperr.ErrNotFound)

	require.NoError(t, e.svc.DeleteApprovalPolicy(e.ctx, p, got.ID))
	assert.ErrorIs(t, e.svc.DeleteApprovalPolicy(e.ctx, p, got.ID), apperr.ErrNotFound)
	for _, action := range []string{"approval_policy.create", "approval_policy.update", "approval_policy.delete"} {
		assert.Equal(t, 1, e.auditCount(t, action, got.ID), action)
	}
	assert.Equal(t, 1, e.auditCount(t, "approval_policy.create", created.ID))
	var data string
	require.NoError(t, e.pool.QueryRow(e.ctx,
		`SELECT data::text FROM audit_event WHERE org_id = $1 AND action = 'approval_policy.create' AND target_id = $2`,
		e.org, created.ID.String()).Scan(&data))
	assert.JSONEq(t, `{"name":"a","mode":"manual","break_glass":true}`, data)
}

func TestApprovalPolicyValidation(t *testing.T) {
	e := setup(t)
	res := dbtest.Resource(t, e.pool, e.org, "web", nil)
	nine := map[string]string{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		nine[k] = "v"
	}
	for name, mut := range map[string]func(*access.ApprovalPolicy){
		"both selectors":       func(p *access.ApprovalPolicy) { p.Selector.ResourceIDs = []uuid.UUID{res} },
		"empty selector":       func(p *access.ApprovalPolicy) { p.Selector = access.Selector{} },
		"nine labels":          func(p *access.ApprovalPolicy) { p.Selector.Labels = nine },
		"bad label key":        func(p *access.ApprovalPolicy) { p.Selector.Labels = map[string]string{"Bad Key": "v"} },
		"bad label value":      func(p *access.ApprovalPolicy) { p.Selector.Labels = map[string]string{"env": "a b"} },
		"101 resources":        func(p *access.ApprovalPolicy) { p.Selector = access.Selector{ResourceIDs: make([]uuid.UUID, 101)} },
		"empty requesters":     func(p *access.ApprovalPolicy) { p.Requesters = access.Subjects{} },
		"approvers all_users":  func(p *access.ApprovalPolicy) { p.Approvers = access.Subjects{AllUsers: true} },
		"approvers all_agents": func(p *access.ApprovalPolicy) { p.Approvers = access.Subjects{AllAgents: true} },
		"mode x":               func(p *access.ApprovalPolicy) { p.Mode = "x" },
		"15 days":              func(p *access.ApprovalPolicy) { p.MaxDuration = 15 * 24 * time.Hour },
		"30 seconds":           func(p *access.ApprovalPolicy) { p.MaxDuration = 30 * time.Second },
		"empty name":           func(p *access.ApprovalPolicy) { p.Name = "" },
		"long name":            func(p *access.ApprovalPolicy) { p.Name = strings.Repeat("a", 64) },
	} {
		p := e.policy(t)
		mut(&p)
		_, err := e.svc.CreateApprovalPolicy(e.ctx, e.principal(), p)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
	ok := e.policy(t)
	ok.MaxDuration = 14 * 24 * time.Hour
	_, err := e.svc.CreateApprovalPolicy(e.ctx, e.principal(), ok)
	assert.NoError(t, err, "14 days is the upper bound")
}

func TestApprovalPolicySubjectsInOrg(t *testing.T) {
	e := setup(t)
	other := dbtest.Org(t, e.pool)
	foreignUser := dbtest.User(t, e.pool, other, "member")
	foreignTeam := dbtest.Team(t, e.pool, other)
	foreignRes := dbtest.Resource(t, e.pool, other, "db", nil)
	for name, mut := range map[string]func(*access.ApprovalPolicy){
		"requester user": func(p *access.ApprovalPolicy) { p.Requesters = access.Subjects{Users: []uuid.UUID{foreignUser}} },
		"requester team": func(p *access.ApprovalPolicy) { p.Requesters = access.Subjects{Teams: []uuid.UUID{foreignTeam}} },
		"approver user": func(p *access.ApprovalPolicy) {
			p.Approvers = access.Subjects{Users: []uuid.UUID{e.owner, foreignUser}}
		},
		"approver team": func(p *access.ApprovalPolicy) { p.Approvers = access.Subjects{Teams: []uuid.UUID{foreignTeam}} },
		"resource":      func(p *access.ApprovalPolicy) { p.Selector = access.Selector{ResourceIDs: []uuid.UUID{foreignRes}} },
	} {
		p := e.policy(t)
		mut(&p)
		_, err := e.svc.CreateApprovalPolicy(e.ctx, e.principal(), p)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}

	own := dbtest.Resource(t, e.pool, e.org, "web", nil)
	ok := e.policy(t)
	ok.Name = "ids"
	ok.Selector = access.Selector{ResourceIDs: []uuid.UUID{own, own}}
	ok.Approvers = access.Subjects{Users: []uuid.UUID{e.owner, e.owner}, Teams: []uuid.UUID{dbtest.Team(t, e.pool, e.org)}}
	_, err := e.svc.CreateApprovalPolicy(e.ctx, e.principal(), ok)
	require.NoError(t, err, "duplicate ids of the own org are fine")

	_, err = e.svc.CreateApprovalPolicy(e.ctx, e.principal(), e.policy(t))
	require.NoError(t, err)
	_, err = e.svc.CreateApprovalPolicy(e.ctx, e.principal(), e.policy(t))
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists, "duplicate name")

	// a policy of org B is invisible to org A
	list, err := e.svc.ListApprovalPolicies(e.ctx, other)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestApprovalPolicyJSONShape(t *testing.T) {
	e := setup(t)
	p := e.policy(t)
	p.Requesters = access.Subjects{Users: []uuid.UUID{e.owner}, AllAgents: true}
	got, err := e.svc.CreateApprovalPolicy(e.ctx, e.principal(), p)
	require.NoError(t, err)
	var sel, req, apr string
	require.NoError(t, e.pool.QueryRow(e.ctx,
		`SELECT resource_selector::text, requesters::text, approvers::text FROM approval_policy WHERE id = $1`, got.ID).Scan(&sel, &req, &apr))
	assert.JSONEq(t, `{"labels":{"env":"prod"}}`, sel)
	assert.JSONEq(t, `{"users":["`+e.owner.String()+`"],"all_agents":true}`, req)
	assert.JSONEq(t, `{}`, apr)

	list, err := e.svc.ListApprovalPolicies(e.ctx, e.org)
	require.NoError(t, err)
	assert.Equal(t, got, list[0], "round trip")
}
