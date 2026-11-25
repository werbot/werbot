package tunnel

import (
	"context"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// StartSession passes the paths the gateway checked; none stay nil.
func TestStartSessionPaths(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	b.f.with(func(f *fake) { f.owned, f.ownedType = true, "ssh" })
	req := &gatewayv1.StartSessionRequest{UserId: uuid.NewString(), ResourceId: uuid.NewString(), ClientIp: "10.1.2.3",
		Kind: "exec", Command: "cat /etc/hosts", Classes: []string{"read"}, Paths: []string{"/etc/hosts"}}
	if _, err := g.client.StartSession(ctx, connect.NewRequest(req)); err != nil {
		t.Fatal(err)
	}
	req.Paths = nil
	if _, err := g.client.StartSession(ctx, connect.NewRequest(req)); err != nil {
		t.Fatal(err)
	}
	b.f.with(func(f *fake) {
		if len(f.starts) != 2 || !reflect.DeepEqual(f.starts[0].Paths, []string{"/etc/hosts"}) || f.starts[1].Paths != nil {
			t.Fatalf("Start paths %+v", f.starts)
		}
	})
}

// resultSessions records the result every End gets.
type resultSessions struct {
	*fake
	results []*SessionResult
}

func (s *resultSessions) End(ctx context.Context, orgID, instanceID, id uuid.UUID, reason string, c *Counters, r *SessionResult) error {
	s.results = append(s.results, r)
	return s.fake.End(ctx, orgID, instanceID, id, reason, c, r)
}

// A console session ends with its last command, classes and paths; an end
// without a command carries no result.
func TestEndSessionConsoleResult(t *testing.T) {
	s := &resultSessions{fake: newFake()}
	h := &Handler{d: Deps{Sessions: s}, log: slog.New(slog.DiscardHandler)}
	gctx := withGateway(ctx, Gateway{ID: uuid.New(), InstanceID: uuid.New(), OrgID: uuid.New()})
	id := uuid.NewString()
	for _, req := range []*gatewayv1.EndSessionRequest{
		{SessionId: id, Reason: "closed", Command: "ls", Classes: []string{"read"}, Paths: []string{"/srv"}},
		{SessionId: id, Reason: "closed"},
	} {
		if _, err := h.EndSession(gctx, connect.NewRequest(req)); err != nil {
			t.Fatal(err)
		}
	}
	want := []*SessionResult{{Command: "ls", Classes: []string{"read"}, Paths: []string{"/srv"}}, nil}
	if !reflect.DeepEqual(s.results, want) {
		t.Fatalf("End results %+v, want %+v", s.results, want)
	}
}

// A console line denial reaches Deny with its session id, classes, paths and
// reasons; an invalid session id rejects the whole heartbeat (Ruling C12).
func TestHeartbeatCommandDenied(t *testing.T) {
	b := newBench(t, nil)
	sid := uuid.New()
	d := &gatewayv1.Denial{SessionId: sid.String(), Resource: "db", Kind: "terminal", Target: "cat /etc/shadow", Reason: "denied",
		Classes: []string{"read", "secrets"}, Paths: []string{"/etc/shadow"}, Reasons: []string{"path:x"}}
	c := b.instance(t, b.org, uuid.New()).connect(t, &gatewayv1.Heartbeat{Version: "1.0", Denials: []*gatewayv1.Denial{d}})
	c.next(t, 2*time.Second)
	eventually(t, func() (ok bool) {
		b.f.with(func(f *fake) { ok = len(f.beat.denials) == 1 })
		return ok
	}, "command denial")
	b.f.with(func(f *fake) {
		want := Denial{SessionID: &sid, Resource: "db", Kind: "terminal", Target: "cat /etc/shadow", Reason: "denied",
			Classes: []string{"read", "secrets"}, Paths: []string{"/etc/shadow"}, Reasons: []string{"path:x"}}
		if got := f.beat.denials[0].denials; len(got) != 1 || !reflect.DeepEqual(got[0], want) {
			t.Fatalf("Deny(%+v), want %+v", got, want)
		}
	})

	bad := proto.CloneOf(d)
	bad.SessionId = "x"
	hb := &gatewayv1.Heartbeat{Denials: []*gatewayv1.Denial{d, bad}}
	if err := b.instance(t, b.org, uuid.New()).connect(t, hb).closed(t, 2*time.Second); err.Code() != connect.CodeInvalidArgument {
		t.Fatalf("invalid session_id: %v", err)
	}
	b.f.with(func(f *fake) {
		if len(f.beat.denials) != 1 {
			t.Fatalf("a rejected heartbeat reached Deny: %+v", f.beat.denials)
		}
	})
}

// The stage 11b tunnel fields keep their bounds (Rulings G11, G12).
func TestTunnelValidationStage11b(t *testing.T) {
	id := uuid.NewString()
	start := func(paths []string) *gatewayv1.StartSessionRequest {
		return &gatewayv1.StartSessionRequest{UserId: id, ResourceId: id, ClientIp: "10.0.0.1", Kind: "exec", Paths: paths}
	}
	end := func(mut func(*gatewayv1.EndSessionRequest)) *gatewayv1.EndSessionRequest {
		r := &gatewayv1.EndSessionRequest{SessionId: id, Reason: "closed", Command: fill(65536),
			Classes: []string{"read", "write"}, Paths: slices.Repeat([]string{fill(1024)}, 256)}
		mut(r)
		return r
	}
	denial := func(mut func(*gatewayv1.Denial)) *gatewayv1.Denial {
		d := &gatewayv1.Denial{SessionId: id, Reason: "denied", Target: fill(1024),
			Classes: []string{"read", "write", "destructive", "privilege", "secrets", "egress", "opaque"},
			Paths:   slices.Repeat([]string{fill(1024)}, 16), Reasons: slices.Repeat([]string{fill(128)}, 16)}
		mut(d)
		return d
	}
	valid := []proto.Message{start(slices.Repeat([]string{fill(1024)}, 256)), start(nil), end(func(*gatewayv1.EndSessionRequest) {}),
		denial(func(*gatewayv1.Denial) {}), denial(func(d *gatewayv1.Denial) { d.SessionId = "" })}
	for i, m := range valid {
		if err := protovalidate.Validate(m); err != nil {
			t.Fatalf("valid %d: %v", i, err)
		}
	}
	for name, m := range map[string]proto.Message{
		"start 257 paths":   start(slices.Repeat([]string{"/a"}, 257)),
		"start long path":   start([]string{fill(1025)}),
		"end long command":  end(func(r *gatewayv1.EndSessionRequest) { r.Command = fill(65537) }),
		"end class":         end(func(r *gatewayv1.EndSessionRequest) { r.Classes = []string{"exec"} }),
		"end 8 classes":     end(func(r *gatewayv1.EndSessionRequest) { r.Classes = slices.Repeat([]string{"read"}, 8) }),
		"end 257 paths":     end(func(r *gatewayv1.EndSessionRequest) { r.Paths = slices.Repeat([]string{"/a"}, 257) }),
		"end long path":     end(func(r *gatewayv1.EndSessionRequest) { r.Paths = []string{fill(1025)} }),
		"denial session":    denial(func(d *gatewayv1.Denial) { d.SessionId = "x" }),
		"denial class":      denial(func(d *gatewayv1.Denial) { d.Classes = []string{"exec"} }),
		"denial 8 classes":  denial(func(d *gatewayv1.Denial) { d.Classes = slices.Repeat([]string{"read"}, 8) }),
		"denial 17 paths":   denial(func(d *gatewayv1.Denial) { d.Paths = slices.Repeat([]string{"/a"}, 17) }),
		"denial long path":  denial(func(d *gatewayv1.Denial) { d.Paths = []string{fill(1025)} }),
		"denial 17 reasons": denial(func(d *gatewayv1.Denial) { d.Reasons = slices.Repeat([]string{"path:x"}, 17) }),
		"denial long reason": denial(func(d *gatewayv1.Denial) {
			d.Reasons = []string{strings.Repeat("r", 129)}
		}),
	} {
		if protovalidate.Validate(m) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
