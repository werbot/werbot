package tunnel

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func TestRequestApproval(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	b.f.with(func(f *fake) { f.owned = true })
	user, res, run := uuid.New(), uuid.New(), uuid.New()
	req := &gatewayv1.RequestApprovalRequest{UserId: user.String(), ResourceId: res.String(), AgentRunId: run.String(),
		Command: "DROP TABLE users", Classes: []string{"destructive"}, Reasons: []string{"drops a table"}}
	resp, err := g.client.RequestApproval(ctx, connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	b.f.approvals.with(func(a *fakeApprovals) {
		if len(a.requests) != 1 {
			t.Fatalf("Request calls = %d, want 1", len(a.requests))
		}
		r := a.requests[0]
		if r.OrgID != b.org || r.InstanceID != g.InstanceID || r.UserID != user || r.ResourceID != res || r.AgentRunID == nil || *r.AgentRunID != run ||
			r.Command != "DROP TABLE users" || !slices.Equal(r.Classes, []string{"destructive"}) || !slices.Equal(r.Reasons, []string{"drops a table"}) {
			t.Fatalf("Request(%+v), want org and instance from the certificate and the request fields", r)
		}
	})
	m := resp.Msg
	if _, err := uuid.Parse(m.GetApprovalId()); err != nil || m.GetExpiresAt().AsTime().Unix() != 1767225600 ||
		m.GetUrl() != "https://werbot.test/requests?id="+m.GetApprovalId() {
		t.Fatalf("response %+v", m)
	}

	req.AgentRunId = ""
	if _, err := g.client.RequestApproval(ctx, connect.NewRequest(req)); err != nil {
		t.Fatal(err)
	}
	b.f.with(func(f *fake) { f.owned = false })
	_, err = g.client.RequestApproval(ctx, connect.NewRequest(req))
	wantCode(t, err, connect.CodePermissionDenied, "")
	b.f.approvals.with(func(a *fakeApprovals) {
		if len(a.requests) != 2 || a.requests[1].AgentRunID != nil {
			t.Fatalf("Request calls %+v: want a nil agent run, none for a resource outside the pool", a.requests)
		}
	})
}

func TestRequestApprovalValidation(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	b.f.with(func(f *fake) { f.owned = true })
	id := uuid.NewString()
	for name, req := range map[string]*gatewayv1.RequestApprovalRequest{
		"empty command":    {UserId: id, ResourceId: id},
		"long command":     {UserId: id, ResourceId: id, Command: strings.Repeat("a", 16385)},
		"user id":          {UserId: "x", ResourceId: id, Command: "ls"},
		"agent run id":     {UserId: id, ResourceId: id, AgentRunId: "x", Command: "ls"},
		"too many classes": {UserId: id, ResourceId: id, Command: "ls", Classes: make([]string, 33)},
	} {
		_, err := g.client.RequestApproval(ctx, connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s: err = %v, want InvalidArgument", name, err)
		}
	}
	if _, err := g.client.RequestApproval(ctx, connect.NewRequest(&gatewayv1.RequestApprovalRequest{
		UserId: id, ResourceId: id, Command: strings.Repeat("a", 16384)})); err != nil {
		t.Fatalf("16384 bytes: %v", err)
	}
}

func TestGetApprovalOtherInstance(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	mine, other := uuid.New(), uuid.New()
	b.f.approvals.with(func(a *fakeApprovals) { a.byID[mine] = Approval{ID: mine, Status: "approved"} })

	resp, err := g.client.GetApproval(ctx, connect.NewRequest(&gatewayv1.GetApprovalRequest{ApprovalId: mine.String()}))
	if err != nil || resp.Msg.GetStatus() != "approved" {
		t.Fatalf("GetApproval = %v, %v; want approved", resp, err)
	}
	_, err = g.client.GetApproval(ctx, connect.NewRequest(&gatewayv1.GetApprovalRequest{ApprovalId: other.String()}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = g.client.GetApproval(ctx, connect.NewRequest(&gatewayv1.GetApprovalRequest{ApprovalId: "x"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	b.f.approvals.with(func(a *fakeApprovals) {
		if len(a.gets) != 2 || a.gets[0] != [3]uuid.UUID{b.org, g.InstanceID, mine} || a.gets[1][1] != g.InstanceID {
			t.Fatalf("Get calls %v, want org and instance from the certificate", a.gets)
		}
	})
}

func TestApprovalDecidedDelivered(t *testing.T) {
	b := newBench(t, nil)
	pool := uuid.New()
	g, peer := b.instance(t, b.org, pool), b.instance(t, b.org, pool)
	approved, pending := uuid.New(), uuid.New()
	b.f.approvals.with(func(a *fakeApprovals) {
		a.byID[approved] = Approval{ID: approved, Status: "approved"}
		a.byID[pending] = Approval{ID: pending, Status: "pending"}
	})
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)
	peerMsgs, peerErr := rawConn(t, peer)
	firstSnapshot(t, peerMsgs)

	b.hub.ApprovalDecided(g.InstanceID.String() + ":" + pending.String())
	b.hub.ApprovalDecided(uuid.NewString() + ":" + approved.String())
	b.hub.ApprovalDecided("garbage")
	b.hub.ApprovalDecided(g.InstanceID.String() + ":garbage")
	b.hub.ApprovalDecided(g.InstanceID.String() + ":" + approved.String())
	got := nextDecided(t, msgs, errc, time.Second)
	if got == nil || got.GetApprovalId() != approved.String() || got.GetStatus() != "approved" {
		t.Fatalf("approval_decided = %v, want %s approved", got, approved)
	}
	if extra := nextDecided(t, msgs, errc, 300*time.Millisecond); extra != nil {
		t.Fatalf("unexpected %v: pending, foreign and malformed decisions send nothing", extra)
	}
	if extra := nextDecided(t, peerMsgs, peerErr, 100*time.Millisecond); extra != nil {
		t.Fatalf("another instance got %v", extra)
	}
}

func TestApprovalDecidedDoesNotBlockListen(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	hold := make(chan struct{})
	b.f.approvals.with(func(a *fakeApprovals) { a.hold = hold })
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)

	payload := g.InstanceID.String() + ":" + uuid.NewString()
	start := time.Now()
	for range 1000 {
		b.hub.ApprovalDecided(payload)
	}
	if d := time.Since(start); d >= 100*time.Millisecond {
		t.Fatalf("1000 ApprovalDecided took %v with Get hanging", d)
	}
	close(hold)
	select {
	case err := <-errc:
		wantCode(t, err, connect.CodeUnavailable, "overflow")
	case <-time.After(2 * time.Second):
		t.Fatal("stream not closed after the control queue overflowed")
	}
	b.f.approvals.with(func(a *fakeApprovals) {
		if n := len(a.gets); n > ControlQueueSize+1 {
			t.Fatalf("Get calls = %d, want at most the queue plus the one in flight", n)
		}
	})
}

func TestResendDecisionsOnConnect(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	a1, a2 := uuid.New(), uuid.New()
	b.f.approvals.with(func(a *fakeApprovals) {
		a.decided = []Approval{{ID: a1, Status: "approved"}, {ID: a2, Status: "denied"}}
	})
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)
	for _, want := range [][2]string{{a1.String(), "approved"}, {a2.String(), "denied"}} {
		got := nextDecided(t, msgs, errc, time.Second)
		if got == nil || got.GetApprovalId() != want[0] || got.GetStatus() != want[1] {
			t.Fatalf("approval_decided = %v, want %v", got, want)
		}
	}
	b.f.approvals.with(func(a *fakeApprovals) {
		if len(a.since) != 1 {
			t.Fatalf("DecidedSince calls = %d, want 1", len(a.since))
		}
		if d := time.Since(a.since[0]) - ApprovalResendWindow; d < 0 || d > 5*time.Second {
			t.Fatalf("since = now-%v, want now-%v", time.Since(a.since[0]), ApprovalResendWindow)
		}
		a.decided, a.err = nil, errors.New("database down")
		a.byID[a1] = Approval{ID: a1, Status: "approved"}
	})

	msgs, errc = rawConn(t, g)
	firstSnapshot(t, msgs)
	b.hub.ApprovalDecided(g.InstanceID.String() + ":" + a1.String())
	if got := nextDecided(t, msgs, errc, time.Second); got == nil || got.GetApprovalId() != a1.String() {
		t.Fatalf("after a failed resend the stream must live: got %v", got)
	}
}
