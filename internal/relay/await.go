package relay

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/pgnotify"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Observe opens a live view of the terminal session sessionID for the
// browser of p: the gateway running it opens the stream within the await
// timeout or ErrAwaitTimeout.
func (r *Relay) Observe(ctx context.Context, p principal.Principal, sessionID uuid.UUID) (Pipe, error) {
	return r.open(ctx, func(target uuid.UUID) (uuid.UUID, uuid.UUID, error) {
		return r.tickets.CreateObserve(ctx, p, sessionID, target)
	})
}

// Terminal opens a web terminal on resource with a cols×rows window for the
// browser of p, like Observe.
func (r *Relay) Terminal(ctx context.Context, p principal.Principal, resource string, cols, rows int) (Pipe, error) {
	return r.open(ctx, func(target uuid.UUID) (uuid.UUID, uuid.UUID, error) {
		return r.tickets.CreateTerminal(ctx, p, resource, cols, rows, target)
	})
}

// open takes a slot, issues a ticket addressed to this replica and waits for
// its gateway stream; the returned Pipe holds the slot until it ends.
func (r *Relay) open(ctx context.Context, create func(target uuid.UUID) (ticketID, instanceID uuid.UUID, err error)) (Pipe, error) {
	if !r.registered.Load() {
		return nil, errNotRunning
	}
	if !r.acquire() {
		r.reject("limit")
		return nil, ErrLimit
	}
	ticketID, instanceID, err := create(r.cfg.InstanceID)
	if err != nil {
		r.release()
		return nil, err
	}
	s, err := r.await(ctx, ticketID, instanceID)
	if err != nil {
		r.release()
		return nil, err
	}
	return s, nil
}

// await registers the wait for ticketID, only then announces the ticket to
// its gateway instance, and waits for the stream.
func (r *Relay) await(ctx context.Context, ticketID, instanceID uuid.UUID) (*stream, error) {
	ch := make(chan *stream, 1)
	r.mu.Lock()
	r.waits[ticketID] = ch
	r.mu.Unlock()
	err := r.notify(ctx, instanceID, ticketID)
	if err == nil {
		t := time.NewTimer(r.awaitTimeout)
		defer t.Stop()
		select {
		case s := <-ch:
			return s, nil
		case <-t.C:
			err = ErrAwaitTimeout
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	r.mu.Lock()
	_, waiting := r.waits[ticketID]
	delete(r.waits, ticketID)
	r.mu.Unlock()
	if !waiting { // handed over while giving up: take it
		return <-ch, nil
	}
	if errors.Is(err, ErrAwaitTimeout) {
		r.reject("timeout")
	}
	return nil, err
}

// notify sends werbot_stream_open "<instance_id>:<ticket_id>".
func (r *Relay) notify(ctx context.Context, instanceID, ticketID uuid.UUID) error {
	return dbx.System(ctx, r.pool, dbx.ReasonCluster, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "SELECT pg_notify($1, $2)", pgnotify.StreamOpen, instanceID.String()+":"+ticketID.String())
		return err
	})
}

// handOff gives p to the browser waiting for ticketID and blocks until that
// stream ends or ctx does; with no waiter p is closed not_found. After
// closeAll p is closed error and the waiter ends on its timeout or context.
func (r *Relay) handOff(ctx context.Context, ticketID uuid.UUID, p Pipe) {
	r.mu.Lock()
	if r.stopping {
		r.mu.Unlock()
		_ = p.Close(ReasonError)
		return
	}
	ch, ok := r.waits[ticketID]
	delete(r.waits, ticketID)
	var s *stream
	if ok {
		s = &stream{Pipe: p, r: r, done: make(chan struct{})}
		r.live[s] = struct{}{}
		ch <- s // buffered: never blocks
	}
	r.mu.Unlock()
	if !ok {
		_ = p.Close(ReasonNotFound)
		return
	}
	select {
	case <-s.done:
	case <-ctx.Done():
	}
	s.detach()
}

// acquire takes a stream slot unless the replica holds maxStreams.
func (r *Relay) acquire() bool {
	if r.active.Add(1) > int64(r.maxStreams) {
		r.active.Add(-1)
		return false
	}
	r.slots.Add(1)
	return true
}

func (r *Relay) release() {
	r.active.Add(-1)
	r.slots.Add(-1)
}

// reject counts a refused stream: limit, origin, ticket, timeout or unavailable.
func (r *Relay) reject(reason string) {
	r.reg.Counter("werbot_relay_rejected_total", "Browser streams refused by this replica.", "reason", reason).Inc()
}

// track adds c to the streams closed on shutdown; false once closeAll ran.
func (r *Relay) track(c closer) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.stopping {
		r.live[c] = struct{}{}
	}
	return !r.stopping
}

func (r *Relay) untrack(c closer) {
	r.mu.Lock()
	delete(r.live, c)
	r.mu.Unlock()
}

// closeAll ends every live stream with ReasonError (shutdown); streams
// arriving after it are closed the same way.
func (r *Relay) closeAll() {
	r.mu.Lock()
	r.stopping = true
	live := make([]closer, 0, len(r.live))
	for c := range r.live {
		live = append(live, c)
	}
	r.mu.Unlock()
	for _, c := range live {
		_ = c.Close(ReasonError)
	}
}

// stream is the browser side of a handed-over gateway stream: it holds the
// browser's slot until Close, a failed Read or detach, and stops reaching the
// gateway stream once its owner (Bridge or the Pipe handler) has returned.
type stream struct {
	Pipe
	r      *Relay
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	reason string // set by Close
	gone   bool   // the gateway stream's owner returned
}

// Read fails once the gateway stream ends; after a local Close the error
// carries that close reason.
func (s *stream) Read() ([]byte, int, int, error) {
	d, c, rows, err := s.Pipe.Read()
	if err == nil {
		return d, c, rows, nil
	}
	s.finish()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reason != "" {
		return nil, 0, 0, CloseError{Reason: s.reason}
	}
	return nil, 0, 0, err
}

func (s *stream) Write(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gone || s.reason != "" {
		return errPipeClosed
	}
	return s.Pipe.Write(b)
}

func (s *stream) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gone || s.reason != "" {
		return errPipeClosed
	}
	return s.Pipe.Resize(cols, rows)
}

func (s *stream) Close(reason string) error {
	s.mu.Lock()
	var err error
	if s.reason == "" {
		s.reason = reason
		if !s.gone {
			err = s.Pipe.Close(reason)
		}
	}
	s.mu.Unlock()
	s.finish()
	return err
}

// detach marks the gateway stream's owner gone; nothing reaches it after.
func (s *stream) detach() {
	s.mu.Lock()
	s.gone = true
	s.mu.Unlock()
	s.finish()
}

// finish ends the stream once: releases the slot and wakes its owner.
func (s *stream) finish() {
	s.once.Do(func() {
		s.r.untrack(s)
		s.r.release()
		close(s.done)
	})
}
