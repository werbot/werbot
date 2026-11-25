package tunnel

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

// open_stream reaches only the named instance; an unknown instance or a
// malformed payload sends nothing, and the log names neither payload nor ticket.
func TestHubStreamOpen(t *testing.T) {
	var logs syncBuffer
	b, _ := newRealtimeBench(t, &logs)
	pool := uuid.New()
	g1, g2 := b.instance(t, b.org, pool), b.instance(t, b.org, pool)
	msgs1, err1 := rawConn(t, g1)
	firstSnapshot(t, msgs1)
	msgs2, err2 := rawConn(t, g2)
	firstSnapshot(t, msgs2)

	ticket := uuid.New()
	b.hub.StreamOpen(g1.InstanceID.String() + ":" + ticket.String())
	m := nextControl(t, msgs1, err1, time.Second)
	if m.GetOpenStream().GetTicketId() != ticket.String() {
		t.Fatalf("got %v, want open_stream %s", m, ticket)
	}
	if extra := nextControl(t, msgs2, err2, 300*time.Millisecond); extra != nil {
		t.Fatalf("another instance got %v", extra)
	}

	start := time.Now()
	b.hub.StreamOpen(uuid.NewString() + ":" + uuid.NewString())
	b.hub.StreamOpen("garbage")
	b.hub.StreamOpen(g1.InstanceID.String() + ":not-a-ticket")
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Fatalf("StreamOpen took %v", d)
	}
	if extra := nextControl(t, msgs1, err1, 300*time.Millisecond); extra != nil {
		t.Fatalf("unknown instance or malformed payload sent %v", extra)
	}
	out := string(logs.bytes())
	if !strings.Contains(out, "payload_len") || !strings.Contains(out, g1.InstanceID.String()) ||
		strings.Contains(out, "not-a-ticket") || strings.Contains(out, "garbage") || strings.Contains(out, ticket.String()) {
		t.Fatalf("malformed payload log must carry only payload_len and the instance id:\n%s", out)
	}
}

func TestHubSessionTerminate(t *testing.T) {
	b, _ := newRealtimeBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)

	s := uuid.New()
	b.hub.SessionTerminate(uuid.NewString() + ":" + uuid.NewString()) // another instance
	b.hub.SessionTerminate(g.InstanceID.String() + ":" + s.String())
	m := nextControl(t, msgs, errc, time.Second)
	if m.GetTerminate().GetSessionId() != s.String() || m.GetTerminate().GetReason() != "terminated" {
		t.Fatalf("got %v, want terminate{%s, terminated}", m, s)
	}
	if extra := nextControl(t, msgs, errc, 300*time.Millisecond); extra != nil {
		t.Fatalf("a terminate for another instance was delivered: %v", extra)
	}
}

// A terminate that does not fit the control queue closes the stream, so the
// reconnected gateway gets it resent; the listener never blocks.
func TestHubTerminateOverflowClosesStream(t *testing.T) {
	b, rt := newRealtimeBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	hold := make(chan struct{})
	rt.set(func(r *realtimeFake) { r.pendHold = hold }) // the stream stops at its first resend
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)

	start := time.Now()
	for range ControlQueueSize + 1 {
		b.hub.SessionTerminate(g.InstanceID.String() + ":" + uuid.NewString())
	}
	if d := time.Since(start); d >= 100*time.Millisecond {
		t.Fatalf("%d SessionTerminate took %v with the stream stuck", ControlQueueSize+1, d)
	}
	close(hold)
	select {
	case err := <-errc:
		wantCode(t, err, connect.CodeUnavailable, "overflow")
	case <-time.After(2 * time.Second):
		t.Fatal("stream not closed after the control queue overflowed")
	}
}

// A full queue drops open_stream (the browser times out) and keeps the stream.
func TestHubOpenStreamOverflowDrops(t *testing.T) {
	var logs syncBuffer
	b, _ := newRealtimeBench(t, &logs)
	g := b.instance(t, b.org, uuid.New())
	hold := make(chan struct{})
	b.f.approvals.with(func(a *fakeApprovals) { a.hold = hold })
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)
	b.hub.ApprovalDecided(g.InstanceID.String() + ":" + uuid.NewString()) // the stream waits in Get
	eventually(t, func() (ok bool) {
		b.f.approvals.with(func(a *fakeApprovals) { ok = len(a.gets) == 1 })
		return ok
	}, "the stream waiting for an approval")

	ticket := uuid.New()
	start := time.Now()
	for range 1000 {
		b.hub.StreamOpen(g.InstanceID.String() + ":" + ticket.String())
	}
	if d := time.Since(start); d >= 100*time.Millisecond {
		t.Fatalf("1000 StreamOpen took %v with the stream stuck", d)
	}
	close(hold)
	for i := range ControlQueueSize {
		if m := nextControl(t, msgs, errc, time.Second); m.GetOpenStream() == nil {
			t.Fatalf("open_stream %d: got %v", i, m)
		}
	}
	if extra := nextControl(t, msgs, errc, 300*time.Millisecond); extra != nil {
		t.Fatalf("got %v beyond the queue size", extra)
	}
	out := string(logs.bytes())
	if !strings.Contains(out, "stream open dropped") || strings.Contains(out, ticket.String()) {
		t.Fatalf("want a drop warning without the ticket id:\n%s", out)
	}
}

func TestTerminateResentOnConnect(t *testing.T) {
	b, rt := newRealtimeBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	s1, s2 := uuid.New(), uuid.New()
	rt.set(func(r *realtimeFake) { r.pending = []uuid.UUID{s1, s2} })
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)
	for _, want := range []uuid.UUID{s1, s2} {
		m := nextControl(t, msgs, errc, time.Second)
		if m.GetTerminate().GetSessionId() != want.String() || m.GetTerminate().GetReason() != "terminated" {
			t.Fatalf("got %v, want terminate %s", m, want)
		}
	}
	rt.set(func(r *realtimeFake) {
		if len(r.pendCalls) != 1 || r.pendCalls[0] != [2]uuid.UUID{b.org, g.InstanceID} {
			t.Fatalf("PendingTerminations calls %v, want one with the certificate's org and instance", r.pendCalls)
		}
		r.pending, r.pendErr = nil, errors.New("database down")
	})

	msgs, errc = rawConn(t, g)
	firstSnapshot(t, msgs)
	s3 := uuid.New()
	b.hub.SessionTerminate(g.InstanceID.String() + ":" + s3.String())
	if m := nextControl(t, msgs, errc, time.Second); m.GetTerminate().GetSessionId() != s3.String() {
		t.Fatalf("after a failed resend the stream must live: got %v", m)
	}
}

// A lost NOTIFY is recovered by Resync without blocking the listener: the
// read runs on the stream's own goroutine.
func TestTerminateResentOnResync(t *testing.T) {
	b, rt := newRealtimeBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)
	eventually(t, func() (ok bool) {
		rt.set(func(r *realtimeFake) { ok = len(r.pendCalls) == 1 })
		return ok
	}, "the resend after the first snapshot")

	s3, hold := uuid.New(), make(chan struct{})
	rt.set(func(r *realtimeFake) { r.pending, r.pendHold = []uuid.UUID{s3}, hold })
	time.AfterFunc(500*time.Millisecond, func() { close(hold) })
	start := time.Now()
	b.hub.Resync()
	if d := time.Since(start); d >= 100*time.Millisecond {
		t.Fatalf("Resync took %v", d)
	}
	m := nextControl(t, msgs, errc, 2*time.Second)
	if m.GetTerminate().GetSessionId() != s3.String() {
		t.Fatalf("got %v, want terminate %s on the same stream", m, s3)
	}
}

func TestAuthenticateUserWebTicket(t *testing.T) {
	b, rt := newRealtimeBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	web, observe, session := uuid.New(), uuid.New(), uuid.New()
	alice := User{ID: uuid.New(), Login: "alice", TeamIDs: []uuid.UUID{uuid.New()}}
	rt.set(func(r *realtimeFake) {
		r.user = alice
		r.tickets[web] = WebTicket{Mode: "web", Resource: "db", Cols: 120, Rows: 40, ClientIP: netip.MustParseAddr("203.0.113.7")}
		r.tickets[observe] = WebTicket{Mode: "observe", SessionID: &session, ClientIP: netip.MustParseAddr("198.51.100.1")}
	})
	auth := func(ticket string) (*gatewayv1.AuthenticateUserResponse, error) {
		resp, err := g.client.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
			By: &gatewayv1.AuthenticateUserRequest_WebTicket{WebTicket: ticket}, ClientIp: "10.0.0.1"}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}

	m, err := auth(web.String())
	want := &gatewayv1.AuthenticateUserResponse{UserId: alice.ID.String(), Login: "alice", TeamIds: []string{alice.TeamIDs[0].String()},
		Country: "DE", Web: &gatewayv1.WebTicket{Mode: "web", Resource: "db", Cols: 120, Rows: 40, ClientIp: "203.0.113.7"}}
	if err != nil || !proto.Equal(m, want) {
		t.Fatalf("response %v, %v; want alice from the browser's country with the web ticket", m, err)
	}
	m, err = auth(observe.String())
	want.Country, want.Web = "NL", &gatewayv1.WebTicket{Mode: "observe", SessionId: session.String(), ClientIp: "198.51.100.1"}
	if err != nil || !proto.Equal(m, want) {
		t.Fatalf("observe ticket: %v, %v", m, err)
	}
	rt.set(func(r *realtimeFake) {
		if r.webCalls[0] != [3]uuid.UUID{b.org, g.InstanceID, web} {
			t.Fatalf("ByWebTicket(%v), want the caller's org and instance", r.webCalls[0])
		}
	})

	_, err = auth(uuid.NewString())
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = auth("x")
	wantCode(t, err, connect.CodeInvalidArgument, "")
	h, gctx, _ := handlerFor(Deps{Users: rt})
	_, err = h.AuthenticateUser(gctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
		By: &gatewayv1.AuthenticateUserRequest_WebTicket{WebTicket: "x"}, ClientIp: "10.0.0.1"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
}

func TestStartSessionFront(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	b.f.with(func(f *fake) { f.owned, f.ownedType = true, "ssh" })
	req := func(front string) *gatewayv1.StartSessionRequest {
		return &gatewayv1.StartSessionRequest{UserId: uuid.NewString(), ResourceId: uuid.NewString(), ClientIp: "10.1.2.3",
			Kind: "terminal", Front: front}
	}
	for _, front := range []string{"web", "", "ssh"} {
		if _, err := g.client.StartSession(ctx, connect.NewRequest(req(front))); err != nil {
			t.Fatalf("front %q: %v", front, err)
		}
	}
	b.f.with(func(f *fake) { // the session domain stores an empty front as ssh
		if len(f.starts) != 3 || f.starts[0].Front != "web" || f.starts[1].Front != "" || f.starts[2].Front != "ssh" {
			t.Fatalf("Start fronts %+v, want web, empty, ssh", f.starts)
		}
	})
	_, err := g.client.StartSession(ctx, connect.NewRequest(req("tcp")))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	b.f.with(func(f *fake) {
		if len(f.starts) != 3 {
			t.Fatal("Start called for an unknown front")
		}
	})
}

// A replica certificate of the same CA is not a gateway: every tunnel call
// is unauthenticated before the handler runs.
func TestTunnelRejectsServerCert(t *testing.T) {
	b, _ := newRealtimeBench(t, nil)
	cert, _ := b.clientCert(t, tlsca.ServerCN(uuid.New()))
	client := b.clientFor(t, cert, nil)
	_, err := client.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
		By: &gatewayv1.AuthenticateUserRequest_WebTicket{WebTicket: uuid.NewString()}, ClientIp: "10.0.0.1"}))
	wantCode(t, err, connect.CodeUnauthenticated, "")
	st := client.Stream(ctx)
	_ = st.Send(&gatewayv1.StreamRequest{Frame: openFrame(uuid.NewString())})
	_, err = st.Receive()
	wantCode(t, err, connect.CodeUnauthenticated, "")
	b.f.with(func(f *fake) {
		if len(f.auths) != 0 {
			t.Fatalf("Authenticate called %d times for a replica certificate", len(f.auths))
		}
	})
}
