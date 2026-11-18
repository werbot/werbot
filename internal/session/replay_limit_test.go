package session_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/session"
)

// gateStore blocks Get until release is closed or fed, unless open is set.
type gateStore struct {
	session.RecordingStore
	entered chan struct{}
	release chan struct{}
	open    atomic.Bool
}

func (g *gateStore) Get(ctx context.Context, orgID, sessionID uuid.UUID, seq int) (io.ReadCloser, error) {
	if !g.open.Load() {
		g.entered <- struct{}{}
		<-g.release
	}
	return g.RecordingStore.Get(ctx, orgID, sessionID, seq)
}

// busyReplays is a replay route with maxReplays replays held open inside the store.
type busyReplays struct {
	e       recEnv
	gate    *gateStore
	h       http.Handler
	done    chan struct{}
	started sync.WaitGroup
}

func newBusyReplays(t *testing.T) *busyReplays {
	t.Helper()
	e, _ := finishedEnv(t)
	g := &gateStore{RecordingStore: e.store.RecordingStore, entered: make(chan struct{}), release: make(chan struct{})}
	e.store.RecordingStore = g
	mux := http.NewServeMux()
	mux.Handle("GET /api/recordings/{file}", session.NewReplayHandler(session.New(e.pool), e.rec, slog.New(slog.NewTextHandler(t.Output(), nil))))
	b := &busyReplays{e: e, gate: g, done: make(chan struct{}, 4), h: mux}
	t.Cleanup(func() { g.open.Store(true); close(g.release); b.started.Wait() })
	for range 4 {
		b.started.Go(func() {
			w := httptest.NewRecorder()
			b.h.ServeHTTP(w, request(context.WithoutCancel(t.Context()), e.as(e.f.OwnerID), castPath(e.sid)))
			b.done <- struct{}{}
		})
		<-g.entered
	}
	return b
}

func (b *busyReplays) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	b.h.ServeHTTP(w, request(t.Context(), b.e.as(b.e.f.OwnerID), path))
	return w
}

func TestReplayCapPerReplica(t *testing.T) {
	b := newBusyReplays(t)
	w := b.get(t, castPath(b.e.sid))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "replay busy, try again\n", w.Body.String())
	assert.Equal(t, "5", w.Header().Get("Retry-After"))

	b.gate.open.Store(true)      // later Gets pass; the three held ones stay blocked
	b.gate.release <- struct{}{} // one replay finishes and frees its slot
	<-b.done
	w = b.get(t, castPath(b.e.sid))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "chunk-0chunk-1chunk-2", w.Body.String())
}

func TestReplayCapAfterAuthAndVisibility(t *testing.T) {
	b := newBusyReplays(t)
	h := b.h
	for name, tc := range map[string]struct {
		p    bool
		path string
		code int
	}{
		"unauthenticated": {false, castPath(b.e.sid), http.StatusUnauthorized},
		"bad id":          {true, "/api/recordings/nope.cast", http.StatusNotFound},
		"not visible":     {true, castPath(uuid.New()), http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			r := request(t.Context(), nil, tc.path)
			if tc.p {
				r = request(t.Context(), b.e.as(b.e.f.OwnerID), tc.path)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			assert.Equal(t, tc.code, w.Code)
			assert.Empty(t, w.Header().Get("Retry-After"))
		})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request(t.Context(), b.e.as(b.e.f.OtherMemberID), castPath(b.e.sid)))
	assert.Equal(t, http.StatusNotFound, w.Code)
}
