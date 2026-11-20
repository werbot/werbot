package tunnel

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// fakeApprovals implements Approvals and records the calls.
type fakeApprovals struct {
	mu       sync.Mutex
	requests []ApprovalRequest
	gets     [][3]uuid.UUID         // org, instance, id
	byID     map[uuid.UUID]Approval // Get answers; missing: ErrNotFound
	decided  []Approval             // DecidedSince answer
	since    []time.Time
	err      error         // DecidedSince error
	hold     chan struct{} // set: Get waits until it is closed
}

func (a *fakeApprovals) Request(_ context.Context, r ApprovalRequest) (Approval, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, r)
	id := uuid.New()
	return Approval{ID: id, Status: "pending", ExpiresAt: time.Unix(1767225600, 0), URL: "https://werbot.test/requests?id=" + id.String()}, nil
}

func (a *fakeApprovals) Get(ctx context.Context, orgID, instanceID, id uuid.UUID) (Approval, error) {
	a.mu.Lock()
	hold := a.hold
	a.gets = append(a.gets, [3]uuid.UUID{orgID, instanceID, id})
	got, ok := a.byID[id]
	a.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return Approval{}, ctx.Err()
		}
	}
	if !ok {
		return Approval{}, apperr.ErrNotFound
	}
	return got, nil
}

func (a *fakeApprovals) DecidedSince(_ context.Context, _, _ uuid.UUID, since time.Time) ([]Approval, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.since = append(a.since, since)
	return a.decided, a.err
}

// with runs fn under the fake's lock to read or change its state.
func (a *fakeApprovals) with(fn func(a *fakeApprovals)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.byID == nil {
		a.byID = map[uuid.UUID]Approval{}
	}
	fn(a)
}

// rawConn opens a Connect stream of g and returns every message it receives and its final error.
func rawConn(t *testing.T, g *gw) (<-chan *gatewayv1.ConnectResponse, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	st := g.client.Connect(ctx)
	if err := st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{Version: "1.0"}}}); err != nil {
		t.Fatal(err)
	}
	msgs, errc := make(chan *gatewayv1.ConnectResponse, 256), make(chan error, 1)
	go func() {
		for {
			m, err := st.Receive()
			if err != nil {
				errc <- err
				return
			}
			msgs <- m
		}
	}()
	return msgs, errc
}

// nextDecided waits for the next approval_decided, skipping other messages; nil when none came within d.
func nextDecided(t *testing.T, msgs <-chan *gatewayv1.ConnectResponse, errc <-chan error, d time.Duration) *gatewayv1.ApprovalDecided {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case m := <-msgs:
			if a := m.GetApprovalDecided(); a != nil {
				return a
			}
		case err := <-errc:
			t.Fatalf("stream closed: %v", err)
		case <-deadline:
			return nil
		}
	}
}

// firstSnapshot waits for the stream's first message and asserts it is a snapshot.
func firstSnapshot(t *testing.T, msgs <-chan *gatewayv1.ConnectResponse) {
	t.Helper()
	select {
	case m := <-msgs:
		if m.GetSnapshot() == nil {
			t.Fatalf("first message %v, want a snapshot", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no snapshot")
	}
}
