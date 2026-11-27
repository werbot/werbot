package gateway

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// runHooks runs s on a connector that returns the hooks of its context.
func runHooks(t *testing.T, e *pipeEnv, s *Session) SessionHooks {
	t.Helper()
	var h SessionHooks
	capture := funcConn{typ: s.Resource.Type, kinds: []Kind{s.Kind}, serve: func(ctx context.Context, _ *Session) error {
		h = HooksFrom(ctx)
		return nil
	}}
	require.NoError(t, e.p.Run(t.Context(), s, capture))
	return h
}

func TestHooksFromContext(t *testing.T) {
	assert.Equal(t, SessionHooks{}, HooksFrom(t.Context()), "no WithHooks: the zero value")

	e := newPipeEnv(t, 10)
	h := runHooks(t, e, e.open(t, "fake", KindTerminal))
	require.NotNil(t, h.PDP)
	assert.Nil(t, h.Limits, "a person has no agent limits")
	assert.NotNil(t, h.Approve)
	assert.NotNil(t, h.Approver)
	assert.NotNil(t, h.RecordDenial)
	assert.NotNil(t, h.WriteCredential)
	assert.Nil(t, h.Mask, "mask_output agents: a person's output is not masked")
	assert.Equal(t, e.f.Policy().Version(), h.PDP.Version())
	assert.True(t, h.PDP.Decide(AccessRequest{Action: "fake.connect"}).Allow, "the PDP decides for the session's user, resource and address")
	e.f.setPolicy(compileTest(t, e.m, `forbid (principal, action, resource);`, nil, res{"rfake", "fake", "fake"}))
	assert.False(t, h.PDP.Decide(AccessRequest{Action: "fake.connect"}).Allow, "the PDP reads the current snapshot")

	a, _ := newAgentEnv(t)
	limits := AgentLimits{MaxRows: 10, MaxResultBytes: 2048, StatementTimeout: time.Second, MaxQueries: 3}
	a.f.with(func(f *fakeControl) { f.user.Agent.Limits = limits })
	s, err := a.p.Open(t.Context(), a.cmdReq("x", KindExec, "ls"))
	require.NoError(t, err)
	h = runHooks(t, a, s)
	require.NotNil(t, h.Limits)
	assert.Equal(t, limits, *h.Limits)
	require.NotNil(t, h.Mask, "an agent's output is masked")
	assert.Equal(t, "pw=[masked]", string(h.Mask([]byte("pw=s3cret-value"))))
}

func TestHooksApproveQuery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, appr := newApprovalEnv(t, "")
		h := runHooks(t, e, e.open(t, "fake", KindTerminal))
		var got []string
		var notifyErr error
		notify := func(id, url string, timeout time.Duration) error {
			got = append(got, id+" "+url+" "+FormatWait(timeout))
			return notifyErr
		}
		in := ApprovalInput{Command: "update t set a = 1", Classes: []string{"write"}, Reasons: []string{"command:w"}}
		done := make(chan error, 1)
		go func() { done <- h.Approve(t.Context(), in, notify) }()
		synctest.Wait()
		require.Equal(t, []string{"a1 https://werbot.test/requests/a1 5m"}, got)
		appr.decide("approved")
		require.NoError(t, <-done)
		require.Len(t, appr.requests, 1)
		assert.Equal(t, "update t set a = 1", appr.requests[0].Command)
		assert.Equal(t, []string{"command:w"}, appr.requests[0].Reasons)

		appr.status, got = "approved", nil
		require.NoError(t, h.Approve(t.Context(), in, notify))
		assert.Equal(t, []string{"a1 https://werbot.test/requests/a1 0m"}, got, "granted before: the ticket with timeout 0, no wait")

		appr.status = ""
		appr.set("denied")
		require.ErrorIs(t, h.Approve(t.Context(), in, notify), ErrApprovalDenied)
		notifyErr = errors.New("client gone")
		require.ErrorIs(t, h.Approve(t.Context(), in, notify), notifyErr, "a failed notice ends the wait")
		appr.reqErr = errors.New("refused")
		require.ErrorIs(t, h.Approve(t.Context(), in, notify), ErrApprovalDenied)
	})
}

// hungControl is a control plane whose RecordDenial waits until its context ends.
type hungControl struct {
	control
	calls atomic.Int32
}

func (h *hungControl) RecordDenial(ctx context.Context, _ uuid.UUID, _, _ string, _, _ []string) (bool, error) {
	h.calls.Add(1)
	<-ctx.Done()
	return false, ctx.Err()
}

func TestRecordDenialQueueOverflow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctl := &hungControl{}
		stop, stopNow := context.WithCancel(context.Background())
		p := &Pipeline{ctl: ctl, denials: newDenials(metrics.NewRegistry()), stop: stop, log: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))}
		run := uuid.New()
		s := &Session{ID: uuid.New(), Kind: KindStream, User: User{ID: uuid.New(), Agent: &Agent{RunID: run}},
			Resource: Resource{Name: "pg"}, Secret: Secret{Value: []byte("s3cret-value")}}
		record := p.recordDenial(s)
		in := ApprovalInput{Command: "drop table t -- s3cret-value", Classes: []string{"destructive"}, Reasons: []string{"agent:baseline:dangerous"}}
		record(in)
		synctest.Wait() // the drain goroutine holds the first denial in its call
		for range 299 {
			record(in) // never blocks
		}
		ds := p.denials.take()
		require.Len(t, ds, 300-1-agentDenialQueue, "past the queue denials go to the heartbeat")
		for _, d := range ds {
			assert.Equal(t, run.String(), d.GetAgentRunId())
			assert.Equal(t, s.ID.String(), d.GetSessionId())
			assert.Equal(t, "denied", d.GetReason())
			assert.Equal(t, []string{"destructive"}, d.GetClasses())
			if bytes.Contains([]byte(d.GetTarget()), []byte("s3cret-value")) {
				t.Fatal("the denial target holds the session credential")
			}
		}
		assert.Equal(t, int32(1), ctl.calls.Load())
		time.Sleep(agentDenialTimeout) // the call times out and goes to the heartbeat
		synctest.Wait()
		assert.Len(t, p.denials.take(), 1)
		assert.Equal(t, int32(2), ctl.calls.Load())
		stopNow()
		synctest.Wait()
	})
}

func TestRecordDenialPerson(t *testing.T) {
	p := &Pipeline{denials: newDenials(metrics.NewRegistry())}
	s := &Session{ID: uuid.New(), Kind: KindStream, User: User{ID: uuid.New()}, Resource: Resource{Name: "pg"}}
	p.recordDenial(s)(ApprovalInput{Command: "drop table t", Classes: []string{"destructive"}, Reasons: []string{"command:r1"}})
	ds := p.denials.take()
	require.Len(t, ds, 1)
	want := &gatewayv1.Denial{UserId: s.User.ID.String(), Resource: "pg", Kind: "stream", Target: "drop table t", Reason: "denied",
		At: ds[0].GetAt(), SessionId: s.ID.String(), Classes: []string{"destructive"}, Reasons: []string{"command:r1"}}
	assert.True(t, proto.Equal(want, ds[0]), "a person's query denial: %v", ds[0])
}

func TestWithAgentLimits(t *testing.T) {
	agent, run, user := uuid.New(), uuid.New(), uuid.New()
	cert := &sshca.Identity{UserID: user, AgentID: &agent, RunID: &run}
	info := &gatewayv1.AgentInfo{AgentId: agent.String(), RunId: run.String(), SponsorUserId: user.String(),
		Limits: &gatewayv1.AgentLimits{MaxRows: 5, MaxResultBytes: 4096, StatementTimeoutMs: 1500, MaxQueries: 9}}
	u, err := withAgent(User{ID: user}, cert, info)
	require.NoError(t, err)
	assert.Equal(t, AgentLimits{MaxRows: 5, MaxResultBytes: 4096, StatementTimeout: 1500 * time.Millisecond, MaxQueries: 9}, u.Agent.Limits)
}
