package gateway

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

const (
	// webQueue is the tickets waiting for the web front; more are dropped.
	webQueue = 64
	// ticketTTL is session.StreamTicketTTL (the gateway does not import
	// session): a ticket id seen within it is not opened again; maxTickets
	// bounds the ids kept.
	ticketTTL  = 30 * time.Second
	maxTickets = 1024
)

// webControl is what the web front needs of the control plane; *tunnelClient
// implements it.
type webControl interface {
	AuthenticateWebTicket(ctx context.Context, ticket uuid.UUID) (User, WebTicket, error)
	OpenStream(ctx context.Context, ticket uuid.UUID) (relayStream, error)
}

// webFront is the browser entry: for every stream ticket the server announces
// it opens the stream and serves a web terminal or a live view on it.
type webFront struct {
	ctl   webControl
	opens chan uuid.UUID
	log   *slog.Logger

	mu   sync.Mutex
	seen map[uuid.UUID]time.Time // ticket ids taken, by when
}

var _ Front = (*webFront)(nil)

func newWebFront(ctl webControl, logger *slog.Logger) *webFront {
	return &webFront{ctl: ctl, opens: make(chan uuid.UUID, webQueue), log: logger, seen: map[uuid.UUID]time.Time{}}
}

func (*webFront) Name() string { return "web" }

// Serve serves every enqueued ticket in its own goroutine until ctx ends,
// then waits for them; it needs no listener. Its streams end with ctx.
func (f *webFront) Serve(ctx context.Context, _ net.Listener, p *Pipeline) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ticket := <-f.opens:
			wg.Go(func() { f.open(ctx, p, ticket) })
		}
	}
}

// enqueue hands ticket to Serve without blocking the tunnel; a ticket seen
// within ticketTTL is ignored: during a reconnect two Connect streams of this
// instance both deliver it.
func (f *webFront) enqueue(ticket uuid.UUID) {
	if !f.firstSeen(ticket) {
		return
	}
	select {
	case f.opens <- ticket:
	default:
		f.log.Warn("web stream dropped")
	}
}

// firstSeen records ticket and reports whether it is new within ticketTTL;
// at maxTickets the oldest id makes room.
func (f *webFront) firstSeen(ticket uuid.UUID) bool {
	now := time.Now()
	f.mu.Lock()
	defer f.mu.Unlock()
	if at, ok := f.seen[ticket]; ok && now.Sub(at) < ticketTTL {
		return false
	}
	pruneOld(f.seen, func(at time.Time) time.Time { return at }, ticketTTL, maxTickets, now)
	f.seen[ticket] = now
	return true
}

// pruneOld deletes the entries of m that are ttl old at now and, when m
// still holds limit, the oldest one: one more entry then fits.
func pruneOld[K comparable, V any](m map[K]V, at func(V) time.Time, ttl time.Duration, limit int, now time.Time) {
	maps.DeleteFunc(m, func(_ K, v V) bool { return now.Sub(at(v)) >= ttl })
	if len(m) >= limit {
		delete(m, slices.MinFunc(slices.Collect(maps.Keys(m)), func(x, y K) int { return at(m[x]).Compare(at(m[y])) }))
	}
}

// open resolves ticket, opens its stream and serves it by the ticket's mode.
// A ticket the server does not resolve (foreign, spent, expired) opens nothing.
func (f *webFront) open(ctx context.Context, p *Pipeline, ticket uuid.UUID) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // ends the stream
	at := p.now()
	u, wt, err := f.ctl.AuthenticateWebTicket(ctx, ticket)
	if err != nil {
		f.log.Warn("web: ticket refused", "code", connect.CodeOf(err).String())
		return
	}
	st, err := f.ctl.OpenStream(ctx, ticket)
	if err != nil {
		f.log.Warn("web: stream not opened", "code", connect.CodeOf(err).String())
		return
	}
	switch wt.Mode {
	case "web":
		serveTerminal(ctx, p, st, OpenRequest{Front: "web", Kind: KindTerminal,
			Identity: Identity{Method: "web-ticket", Value: ticket.String()}, Handshake: &Handshake{User: u, At: at},
			Resource: wt.Resource, ClientIP: wt.ClientIP, Window: wt.Window})
	case "observe":
		watch(ctx, p, st, wt.SessionID)
	default:
		_ = st.Close("error")
	}
}

// watch streams session id to st until it ends: its window, then its output
// and window changes. Frames from st are read only to see its end; nothing
// reaches the session, and the session learns nothing of the observer.
func watch(ctx context.Context, p *Pipeline, st relayStream, id uuid.UUID) {
	events, leave, err := p.active.Observe(id)
	if err != nil { // errObserverLimit
		_ = st.Close("limit")
		return
	}
	defer leave()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		defer cancel()
		for {
			if _, _, _, err := st.Read(); err != nil {
				return
			}
		}
	}()
	reason := "not_found" // closed before its first event: nothing to watch
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				_ = st.Close(reason)
				return
			}
			reason = "closed"
			if err := sendEvent(st, ev); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func sendEvent(st relayStream, ev ObserveEvent) error {
	if ev.Window.Cols > 0 {
		return st.Resize(ev.Window.Cols, ev.Window.Rows)
	}
	return st.Write(ev.Data)
}

// streamCloseReason is the close reason of a stream whose Open failed with err.
func streamCloseReason(err error) string {
	switch {
	case errors.Is(err, ErrBusy), errors.Is(err, ErrSpoolMax):
		return "limit"
	case errors.Is(err, ErrDenied), errors.Is(err, ErrTargetNotAllowed), errors.Is(err, ErrUnsupportedType),
		errors.Is(err, ErrUnsupportedKind), errors.Is(err, ErrApprovalDenied), errors.Is(err, ErrApprovalTimeout),
		errors.Is(err, ErrShellDenied):
		return "denied"
	}
	return "error"
}
