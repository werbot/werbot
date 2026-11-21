package gateway

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func TestExecPathOnlyDeniesOutside(t *testing.T) {
	m := ids{}
	rule, text := pathRule(m, "p1", "only", "", "~/app/**")
	e := pathsPipeEnv(t, m, pathsPolicies+text, rule)

	_, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "cp ~/app/a /etc/hosts"))
	cd, ok := errors.AsType[*CommandDeniedError](err)
	require.True(t, ok, "%v", err)
	require.Equal(t, "Command denied: path outside allowed folders", cd.Error())
	require.Zero(t, e.startCalls())

	s, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "cat ~/app/a"))
	require.NoError(t, err)
	require.Equal(t, []string{"~/app/a"}, s.Paths)
	require.Equal(t, []string{"~/app/a"}, pathsOf(s.ID), "StartSession gets the paths")
	require.Equal(t, []string{"~/app/a"}, e.trackedReq(s.ID).Paths)
}

func TestExecPathDenyMessage(t *testing.T) {
	m := ids{}
	rule, text := pathRule(m, "p1", "deny", "No /etc", "/etc/**")
	e := pathsPipeEnv(t, m, pathsPolicies+text, rule)
	_, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "cat /etc/hosts"))
	require.EqualError(t, err, "Command denied: No /etc")
	ds := e.p.denials.take()
	require.Len(t, ds, 1)
	assert.Equal(t, "denied", ds[0].GetReason())
	assert.Equal(t, "exec", ds[0].GetKind())
	assert.Equal(t, "cat /etc/hosts", ds[0].GetTarget())
	assert.Empty(t, ds[0].GetSessionId(), "an exec denial names no session")
}

// An unresolved path is outside every only rule; without path rules it only adds opaque.
func TestExecUnresolvedPathWithOnly(t *testing.T) {
	m := ids{}
	rule, text := pathRule(m, "p1", "only", "", "~/app/**")
	e := pathsPipeEnv(t, m, pathsPolicies+text, rule)
	_, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "cat $F"))
	require.EqualError(t, err, "Command denied: path outside allowed folders")

	e = pathsPipeEnv(t, ids{}, pathsPolicies)
	s, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "cat $F"))
	require.NoError(t, err)
	require.Contains(t, e.trackedReq(s.ID).Classes, "opaque")
}

func TestExecPathApprovalWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := ids{}
		rule, text := pathRule(m, "p1", "approval", "", "/srv/**")
		e := pathsPipeEnv(t, m, pathsPolicies+text, rule)
		a := &fakeApprovals{ttl: 5 * time.Minute, statuses: []string{"pending"}}
		e.f.with(func(f *fakeControl) { f.approvals = a })
		time.AfterFunc(10*time.Second, func() { a.decide("approved") })
		var notice bytes.Buffer
		r := e.cmdReq("x", KindExec, "cat /srv/a")
		r.Notice = &notice
		s, err := e.p.Open(t.Context(), r)
		require.NoError(t, err)
		require.Contains(t, notice.String(), "Waiting for approval a1")
		require.Len(t, a.requests, 1)
		require.Equal(t, []string{"path:" + m["p1"].String()}, a.requests[0].Reasons)
		require.Equal(t, "cat /srv/a", a.requests[0].Command, "an exec asks for the command itself")
		require.Equal(t, []string{"/srv/a"}, pathsOf(s.ID))
	})
}

// After the approval the command is analysed and decided again on the
// snapshot of that time: a path rule added during the wait refuses it.
func TestApprovedPathRecomputedOnNewSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := ids{}
		rule, text := pathRule(m, "p1", "approval", "", "/srv/**")
		e := pathsPipeEnv(t, m, pathsPolicies+text, rule)
		a := &fakeApprovals{ttl: 5 * time.Minute, statuses: []string{"pending"}}
		e.f.with(func(f *fakeControl) { f.approvals = a })
		frozen, frozenText := pathRule(e.m, "p2", "deny", "frozen", "/srv/**")
		time.AfterFunc(10*time.Second, func() {
			e.f.setPolicy(pathsPolicy(t, e.m, "v2", pathsPolicies+text+frozenText, rule, frozen))
			a.decide("approved")
		})
		_, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "cat /srv/a"))
		require.EqualError(t, err, "Command denied: frozen")
		require.Zero(t, e.startCalls())
	})
}

// A console line is analysed from its cwd with home: relative paths and "~"
// globs resolve to absolute paths.
func TestAuthorizeCommandHomeExpansion(t *testing.T) {
	m := ids{}
	only, onlyText := pathRule(m, "p1", "only", "", "~/app/**")
	e := pathsPipeEnv(t, m, pathsPolicies+onlyText, only)
	base := e.consoleBase(t, "x")
	_, req, err := e.p.authorizeCommand(t.Context(), e.p.ctl.Policy(), base, commandInput{"cat x", "/home/u/app", "/home/u"}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"/home/u/app/x"}, req.Paths)
	_, _, err = e.p.authorizeCommand(t.Context(), e.p.ctl.Policy(), base, commandInput{"cat x", "/etc", "/home/u"}, nil)
	require.EqualError(t, err, "Command denied: path outside allowed folders")

	deny, denyText := pathRule(m, "p2", "deny", "secret", "~/secret/**")
	e = pathsPipeEnv(t, m, pathsPolicies+denyText, deny)
	_, _, err = e.p.authorizeCommand(t.Context(), e.p.ctl.Policy(), e.consoleBase(t, "x"), commandInput{"cat secret/x", "/srv", "/srv"}, nil)
	require.EqualError(t, err, "Command denied: secret")
}

// consoleBase is the exec request of u1 on resource name, as a console line starts from.
func (e *pipeEnv) consoleBase(t *testing.T, name string) AccessRequest {
	t.Helper()
	r, ok := e.p.ctl.Policy().Resource(name)
	require.True(t, ok)
	u := e.f.user
	return AccessRequest{User: u, Resource: r, IP: e.req(name, KindExec).ClientIP, Action: r.Type + ".exec"}
}

// A path rule with a bad id, glob, access or effect rejects the snapshot; the
// tunnel keeps the previous Policy.
func TestCompilePolicyRejectsBadPathRule(t *testing.T) {
	id := "6b7c3f62-3f3e-4f0a-9d5e-4c1d2a3b4c5d"
	for name, r := range map[string]*gatewayv1.PathRule{
		"relative glob":  {Id: id, Paths: []string{"app/**"}, Access: "any", Effect: "only"},
		"no globs":       {Id: id, Access: "any", Effect: "only"},
		"bad id":         {Id: "x", Paths: []string{"/srv/**"}, Access: "any", Effect: "only"},
		"unknown access": {Id: id, Paths: []string{"/srv/**"}, Access: "exec", Effect: "only"},
		"unknown effect": {Id: id, Paths: []string{"/srv/**"}, Access: "any", Effect: "allow"},
	} {
		_, err := CompilePolicy(&gatewayv1.Snapshot{PathRules: []*gatewayv1.PathRule{r}})
		assert.Error(t, err, name)
	}
	p, err := CompilePolicy(&gatewayv1.Snapshot{PathRules: []*gatewayv1.PathRule{{Id: id, Paths: []string{"~/app/**", "/srv/*.key"}, Access: "write", Effect: "deny"}}})
	require.NoError(t, err)
	require.Len(t, p.PathRules(), 1)
	assert.Equal(t, id, p.PathRules()[0].ID)

	c := &tunnelClient{log: slog.New(slog.DiscardHandler), changed: make(chan struct{}, 1)}
	require.True(t, c.apply(&gatewayv1.Snapshot{Version: "v1"}))
	require.False(t, c.apply(&gatewayv1.Snapshot{Version: "v2", PathRules: []*gatewayv1.PathRule{{Id: id, Paths: []string{"app/**"}, Access: "any", Effect: "only"}}}))
	assert.Equal(t, "v1", c.policy.Load().Version())
}
