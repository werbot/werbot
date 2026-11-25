package gateway

import (
	"crypto/sha256"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// observeTicket is an observe ticket for session id.
func observeTicket(id uuid.UUID) WebTicket {
	return WebTicket{Mode: "observe", SessionID: id, ClientIP: browserIP}
}

func TestObserveStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		s := w.pipeEnv.open(t, "blocker", KindTerminal) // an SSH terminal
		done := w.runBlocker(t, s)
		st := w.open(t, observeTicket(s.ID))
		synctest.Wait()
		s.Recording.Output([]byte("hello"))
		synctest.Wait()
		assert.Equal(t, []Window{{120, 40}}, st.windows(), "the observer gets the window first")
		assert.Equal(t, "hello", st.output())
		w.p.active.Terminate(s.ID, "terminated")
		<-done
		assert.Equal(t, "closed", st.closedWith(t))
		assert.Equal(t, 1, w.startCalls(), "observing opens no session")
	})
}

func TestObserveIgnoresInput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		rec := &capturedSink{sum: sha256.New()}
		w.p.cfg.Recording = func(uuid.UUID, int, int) RecordingSink { return rec }
		target := w.open(t, terminalTicket("echo"))
		synctest.Wait()
		obs := w.open(t, observeTicket(w.session(t)))
		obs.in <- frame{data: []byte("echo injected\r")}
		obs.in <- frame{cols: 10, rows: 10}
		synctest.Wait()
		assert.Equal(t, "win=120x40\n", target.output(), "the session gets no input and no window from an observer")
		assert.NotContains(t, rec.events, "r:10x10", "the recording sees no observer resize")
		assert.Equal(t, []Window{{120, 40}}, obs.windows())
		target.in <- frame{data: []byte("ok\r")}
		synctest.Wait()
		assert.Equal(t, "ok\r", obs.output(), "the observer gets the session's output only")

		target.in <- browserClosed
		assert.Equal(t, "closed", target.closedWith(t))
		assert.Equal(t, "closed", obs.closedWith(t), "the observer ends with the session")
	})
}

func TestObserveStreamLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		s := w.pipeEnv.open(t, "blocker", KindTerminal)
		for range maxObservers {
			w.observe(t, s.ID)
		}
		st := w.open(t, observeTicket(s.ID))
		assert.Equal(t, "limit", st.closedWith(t), "the 17th observer is refused")
		w.p.abort(s)
	})
}

func TestObserveNotFound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		st := w.open(t, observeTicket(uuid.New())) // not on this instance
		assert.Equal(t, "not_found", st.closedWith(t))
		assert.Empty(t, st.output())
	})
}

func TestWebFrontDedupTicket(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		ticket := w.add(terminalTicket("fake"))
		st := w.stream(t, ticket)
		w.wf.enqueue(ticket) // the same open_stream from a second Connect stream
		assert.Equal(t, "closed", st.closedWith(t))
		synctest.Wait()
		_, opens := w.ctl.calls()
		assert.Equal(t, 1, opens, "a repeated ticket opens one stream")
		assert.Equal(t, 1, w.startCalls(), "and one session")

		time.Sleep(ticketTTL)
		w.stream(t, ticket).closedWith(t)
		assert.Equal(t, 2, w.startCalls(), "after the ticket TTL the id is taken again")
	})

	f := newWebFront(nil, nil)
	first := uuid.New()
	require.True(t, f.firstSeen(first))
	for range maxTickets {
		require.True(t, f.firstSeen(uuid.New()))
	}
	require.Len(t, f.seen, maxTickets, "the set is bounded")
	require.True(t, f.firstSeen(first), "the oldest id was dropped")
}

func TestWebFrontDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		active := w.open(t, terminalTicket("blocker"))
		synctest.Wait()
		id := w.session(t)
		drained := make(chan struct{})
		go func() { defer close(drained); w.p.drain(time.Minute) }()
		synctest.Wait()

		st := w.open(t, terminalTicket("blocker"))
		assert.Equal(t, "error", st.closedWith(t), "a new ticket is refused during the drain")
		assert.Equal(t, "Gateway is not ready, try again later\r\n", st.output())

		<-drained
		assert.Equal(t, "closed", active.closedWith(t))
		assert.Equal(t, []string{"gateway_shutdown"}, w.endedWith(id))
	})
}

func TestWebFrontQueueFull(t *testing.T) {
	logs := &syncBuffer{}
	f := newWebFront(nil, slog.New(slog.NewTextHandler(logs, nil)))
	for range webQueue {
		f.enqueue(uuid.New())
	}
	f.enqueue(uuid.New()) // never blocks the tunnel
	assert.Len(t, f.opens, webQueue)
	assert.Contains(t, logs.String(), "web stream dropped")
}
