package tunnel

import (
	"context"
	"io"
	"log/slog"
	"net/netip"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// The bench fake knows no web tickets and no pending terminations; tests of
// them use realtimeFake.
func (f *fake) ByWebTicket(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (User, WebTicket, error) {
	return User{}, WebTicket{}, apperr.ErrNotFound
}

func (f *fake) PendingTerminations(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

// realtimeFake answers web tickets, pending terminations and bridges over the bench fake.
type realtimeFake struct {
	*fake
	bridges chan bridgeCall

	mu        sync.Mutex
	user      User
	tickets   map[uuid.UUID]WebTicket
	webCalls  [][3]uuid.UUID // org, instance, ticket
	pending   []uuid.UUID
	pendErr   error
	pendCalls [][2]uuid.UUID // org, instance
	pendHold  chan struct{}  // set: PendingTerminations returns once it is closed or ctx ends
}

// bridgeCall is one Bridge; the test ends it by sending its result to done.
type bridgeCall struct {
	gw     Gateway
	ticket uuid.UUID
	p      Pipe
	done   chan error
}

func (r *realtimeFake) ByWebTicket(_ context.Context, orgID, instanceID, ticketID uuid.UUID) (User, WebTicket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webCalls = append(r.webCalls, [3]uuid.UUID{orgID, instanceID, ticketID})
	wt, ok := r.tickets[ticketID]
	if !ok {
		return User{}, WebTicket{}, apperr.ErrNotFound
	}
	return r.user, wt, nil
}

func (r *realtimeFake) PendingTerminations(ctx context.Context, orgID, instanceID uuid.UUID) ([]uuid.UUID, error) {
	r.mu.Lock()
	r.pendCalls = append(r.pendCalls, [2]uuid.UUID{orgID, instanceID})
	ids, err, hold := r.pending, r.pendErr, r.pendHold
	r.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return ids, err
}

func (r *realtimeFake) Bridge(ctx context.Context, gw Gateway, ticketID uuid.UUID, p Pipe) error {
	c := bridgeCall{gw: gw, ticket: ticketID, p: p, done: make(chan error, 1)}
	r.bridges <- c
	select {
	case err := <-c.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// set runs fn under the fake's lock to read or change its realtime state.
func (r *realtimeFake) set(fn func(r *realtimeFake)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn(r)
}

// newRealtimeBench is a bench whose users, sessions and relay are a
// realtimeFake; logs (nil: discarded) gets the handler's log; the country
// of 203.0.113.7 is "DE", of any other address "NL".
func newRealtimeBench(t *testing.T, logs io.Writer) (*bench, *realtimeFake) {
	t.Helper()
	rt := &realtimeFake{bridges: make(chan bridgeCall, 4), tickets: map[uuid.UUID]WebTicket{}}
	b := newBenchTLS(t, nil, nil, func(d *Deps) {
		rt.fake = d.Users.(*fake)
		d.Users, d.Sessions, d.Relay = rt, rt, rt
		d.Country = func(ip netip.Addr) string {
			if ip == netip.MustParseAddr("203.0.113.7") {
				return "DE"
			}
			return "NL"
		}
		if logs != nil {
			d.Logger = slog.New(slog.NewTextHandler(logs, nil))
		}
	})
	return b, rt
}

// nextControl waits for the next open_stream or terminate, skipping other
// messages; nil when none came within d.
func nextControl(t *testing.T, msgs <-chan *gatewayv1.ConnectResponse, errc <-chan error, d time.Duration) *gatewayv1.ConnectResponse {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case m := <-msgs:
			if m.GetOpenStream() != nil || m.GetTerminate() != nil {
				return m
			}
		case err := <-errc:
			t.Fatalf("stream closed: %v", err)
		case <-deadline:
			return nil
		}
	}
}

type streamClient = connect.BidiStreamForClient[gatewayv1.StreamRequest, gatewayv1.StreamResponse]

// dialStream opens TunnelService.Stream as g and sends frame first. The
// stream ends after 10 s, so a step waiting on it fails instead of hanging.
func dialStream(t *testing.T, g *gw, first *gatewayv1.StreamFrame) *streamClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	st := g.client.Stream(ctx)
	if err := st.Send(&gatewayv1.StreamRequest{Frame: first}); err != nil {
		t.Fatal(err)
	}
	return st
}

func openFrame(ticket string) *gatewayv1.StreamFrame {
	return &gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Open{Open: &gatewayv1.StreamOpen{TicketId: ticket}}}
}

func dataFrame(p []byte) *gatewayv1.StreamFrame {
	return &gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Data{Data: p}}
}

func resizeFrame(cols, rows int32) *gatewayv1.StreamFrame {
	return &gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Resize{Resize: &gatewayv1.Resize{Cols: cols, Rows: rows}}}
}

func closeFrame(reason string) *gatewayv1.StreamFrame {
	return &gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Close{Close: &gatewayv1.StreamClose{Reason: reason}}}
}

// nextBridge waits for the next Bridge call.
func (r *realtimeFake) nextBridge(t *testing.T) bridgeCall {
	t.Helper()
	select {
	case c := <-r.bridges:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("no Bridge call")
		return bridgeCall{}
	}
}
