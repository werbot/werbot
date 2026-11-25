package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// approvalPolicies are execTestPolicies plus a rule that asks for an approval
// of write commands; message is its @message line, "" for none.
func approvalPolicies(message string) string {
	return execTestPolicies + `
@id("command:w")` + message + `
@obligation("approval")
forbid (principal, action == Werbot::Action::"execy.exec", resource) when { context.classes.contains("write") }
unless { context.approval_granted };`
}

const askOps = "\n@message(\"ask ops\")"

// newApprovalEnv is a pipeline on approvalPolicies(message) whose approvals
// stay pending and expire after 5 minutes.
func newApprovalEnv(t *testing.T, message string) (*pipeEnv, *fakeApprovals) {
	t.Helper()
	e := newPipeEnv(t, 10)
	e.f.setPolicy(execPolicy(t, e.m, "v1", approvalPolicies(message)))
	a := &fakeApprovals{ttl: 5 * time.Minute, statuses: []string{"pending"}}
	e.f.with(func(f *fakeControl) { f.approvals = a })
	return e, a
}

// openWrite opens an exec of a write command, which needs an approval, and
// returns how long Open took.
func (e *pipeEnv) openWrite(ctx context.Context, notice *bytes.Buffer) (*Session, time.Duration, error) {
	r := e.cmdReq("x", KindExec, "mkdir x")
	if notice != nil {
		r.Notice = notice
	}
	start := time.Now()
	s, err := e.p.Open(ctx, r)
	return s, time.Since(start), err
}

// trackedReq is the request the sweeper re-decides for session id.
func (e *pipeEnv) trackedReq(id uuid.UUID) AccessRequest {
	e.p.active.mu.Lock()
	defer e.p.active.mu.Unlock()
	return e.p.active.sessions[id].req
}

// requireDenial checks the one denial of the last refused Open.
func (e *pipeEnv) requireDenial(t *testing.T, reason string) {
	t.Helper()
	ds := e.p.denials.take()
	require.Len(t, ds, 1)
	require.Equal(t, reason, ds[0].GetReason())
	require.Equal(t, "mkdir x", ds[0].GetTarget())
}

func TestApprovalApprovedViaStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		time.AfterFunc(10*time.Second, func() { a.decide("approved") })
		var notice bytes.Buffer
		s, took, err := e.openWrite(t.Context(), &notice)
		require.NoError(t, err)
		require.Equal(t, 10*time.Second, took)
		require.Equal(t, "Waiting for approval a1, timeout 5m\r\nhttps://werbot.test/requests/a1\r\nApproved\r\n", notice.String())
		require.Equal(t, []approvalRequest{{UserID: e.m["u1"], ResourceID: e.m["rx"], Command: "mkdir x",
			Classes: []string{"write"}, Reasons: []string{"command:w"}}}, a.requests)
		require.Len(t, a.gets, 1, "the decision is read back once the stream hints at it")
		require.Equal(t, 1, e.startCalls())
		require.Equal(t, fmt.Sprintf("%s %s 192.0.2.7", e.m["u1"], e.m["rx"])+startCmd("mkdir x", []string{"write"}), e.lastStart())
		require.True(t, e.trackedReq(s.ID).ApprovalGranted, "the sweeper re-decides the command as approved")
		require.Zero(t, a.waiting(), "the wait unsubscribes")
	})
}

func TestApprovalPollFallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		a.statuses = []string{"pending", "pending", "approved"}
		start := time.Now()
		_, took, err := e.openWrite(t.Context(), nil)
		require.NoError(t, err, "no decision on the stream: the poll finds it")
		require.Equal(t, 45*time.Second, took)
		var at []time.Duration
		for _, g := range a.gets {
			at = append(at, g.Sub(start))
		}
		require.Equal(t, []time.Duration{15 * time.Second, 30 * time.Second, 45 * time.Second}, at)
	})
}

func TestApprovalDenied(t *testing.T) {
	for _, tc := range []struct{ name, status, message, want string }{
		{"denied", "denied", "", "Command denied: approval denied"},
		{"canceled", "canceled", "", "Command denied: approval denied"},
		{"rule message", "denied", askOps, "Command denied: ask ops"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e, a := newApprovalEnv(t, tc.message)
				time.AfterFunc(10*time.Second, func() { a.decide(tc.status) })
				_, took, err := e.openWrite(t.Context(), nil)
				require.ErrorIs(t, err, ErrApprovalDenied)
				require.Equal(t, 10*time.Second, took)
				require.Equal(t, tc.want, openText(err, "x", KindExec, nil))
				require.Zero(t, e.startCalls())
				e.requireDenial(t, "approval_denied")
			})
		})
	}
}

func TestApprovalTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		_, took, err := e.openWrite(t.Context(), nil)
		require.ErrorIs(t, err, ErrApprovalTimeout)
		require.Equal(t, 5*time.Minute, took, "the wait ends at expires_at")
		require.Equal(t, "Approval timed out", openText(err, "x", KindExec, nil))
		require.Zero(t, e.startCalls())
		e.requireDenial(t, "approval_timeout")
		require.Zero(t, a.waiting())

		time.AfterFunc(time.Minute, func() { a.decide("expired") })
		_, took, err = e.openWrite(t.Context(), nil)
		require.ErrorIs(t, err, ErrApprovalTimeout, "an expired approval is a timeout")
		require.Equal(t, time.Minute, took)
	})
}

func TestApprovalServerDownTimesOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		a.getErr = connect.NewError(connect.CodeUnavailable, errors.New("down"))
		_, took, err := e.openWrite(t.Context(), nil)
		require.ErrorIs(t, err, ErrApprovalTimeout)
		require.Equal(t, 5*time.Minute, took)
		require.Contains(t, e.log.String(), "approval_id=a1", "a failed read is logged")
		require.NotContains(t, e.log.String(), "mkdir", "never the command")
	})
}

func TestApprovalLateDecisionAtDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		a.ttl = 5*time.Minute + 10*time.Second                                    // the deadline falls between two polls
		time.AfterFunc(5*time.Minute+5*time.Second, func() { a.set("approved") }) // its hint is lost
		_, took, err := e.openWrite(t.Context(), nil)
		require.NoError(t, err, "the final read at the deadline finds the decision")
		require.Equal(t, 5*time.Minute+10*time.Second, took)
		require.Equal(t, 1, e.startCalls())
	})
}

func TestApprovalDecisionAfterTimeoutIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		_, _, err := e.openWrite(t.Context(), nil)
		require.ErrorIs(t, err, ErrApprovalTimeout)
		a.decide("approved") // nobody waits: dropped without blocking
		synctest.Wait()
		require.Zero(t, e.startCalls())
		require.Zero(t, a.waiting())
	})
}

func TestApprovalClientDisconnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(30*time.Second, cancel)
		_, took, err := e.openWrite(ctx, nil)
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, 30*time.Second, took, "the wait ends with the client")
		require.Zero(t, e.p.active.used(), "the slot is free")
		require.Zero(t, a.waiting(), "the subscription is gone")
		require.Zero(t, e.startCalls())
	})
}

func TestApprovalAbortsOnDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, _ := newApprovalEnv(t, "")
		time.AfterFunc(30*time.Second, func() { e.p.drain(time.Minute) })
		_, took, err := e.openWrite(t.Context(), nil)
		require.ErrorIs(t, err, ErrNotReady)
		require.Equal(t, 30*time.Second, took, "drain ends the wait at once")
		require.Equal(t, "Gateway is not ready, try again later", openText(err, "x", KindExec, nil))
		require.Empty(t, e.p.denials.take(), "not ready is not a denial")
	})
}

// After the approval the command is decided again on the snapshot of that
// moment: a rule added during the wait refuses it with its message.
func TestApprovedButDeniedByNewRule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		frozen := execPolicy(t, e.m, "v2", approvalPolicies("")+`
@id("command:z")
@message("frozen")
forbid (principal, action == Werbot::Action::"execy.exec", resource) when { context.classes.contains("write") };`)
		time.AfterFunc(10*time.Second, func() { e.f.setPolicy(frozen); a.decide("approved") })
		_, _, err := e.openWrite(t.Context(), nil)
		require.ErrorIs(t, err, ErrDenied)
		require.Equal(t, "Command denied: frozen", openText(err, "x", KindExec, nil))
		require.Zero(t, e.startCalls())
		e.requireDenial(t, "denied")
	})
}

// After the approval the connection is decided again too: a resource the user
// lost during the wait refuses the command as a plain deny.
func TestApprovedButConnectRevoked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, a := newApprovalEnv(t, "")
		revoked := execPolicy(t, e.m, "v2", approvalPolicies("")+`
forbid (principal, action, resource == Werbot::Resource::"rx");`)
		time.AfterFunc(10*time.Second, func() { e.f.setPolicy(revoked); a.decide("approved") })
		_, _, err := e.openWrite(t.Context(), nil)
		require.ErrorIs(t, err, ErrDenied)
		require.Equal(t, "Access denied", openText(err, "x", KindExec, nil))
		require.Zero(t, e.startCalls())
	})
}

func TestApprovalRequestUnavailable(t *testing.T) {
	e, a := newApprovalEnv(t, "")
	a.reqErr = connect.NewError(connect.CodeUnavailable, errors.New("down"))
	_, _, err := e.openWrite(t.Context(), nil)
	require.ErrorIs(t, err, ErrNotReady)
	require.Equal(t, "Gateway is not ready, try again later", openText(err, "x", KindExec, nil))
	require.Zero(t, e.startCalls())
}

func TestApprovalRequestRejected(t *testing.T) {
	e, a := newApprovalEnv(t, askOps)
	a.reqErr = connect.NewError(connect.CodeInvalidArgument, errors.New("invalid"))
	var notice bytes.Buffer
	_, _, err := e.openWrite(t.Context(), &notice)
	require.ErrorIs(t, err, ErrApprovalDenied)
	require.Equal(t, "Command denied: approval denied", openText(err, "x", KindExec, nil), "the rule message is for a decision")
	require.Empty(t, notice.String(), "no wait was announced")
	require.Zero(t, e.startCalls())
	e.requireDenial(t, "approval_denied")
}

func TestFormatWait(t *testing.T) {
	for d, want := range map[time.Duration]string{
		5 * time.Minute:                          "5m",
		4*time.Minute + 59600*time.Millisecond:   "5m",
		90 * time.Second:                         "90s",
		10 * time.Minute:                         "10m",
		59*time.Second + 400*time.Millisecond:    "59s",
		time.Hour + 30*time.Second + time.Second: "3631s",
	} {
		require.Equal(t, want, formatWait(d), d.String())
	}
}
