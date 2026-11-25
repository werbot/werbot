package gateway

import (
	"context"
	"crypto/sha256"
	"io"
	"testing"
	"testing/synctest"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// browserClosed is the Read error of a stream the browser closed.
var browserClosed = frame{err: streamCloseError{reason: "closed"}}

func TestWebFrontOpensTerminalSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		rec := &capturedSink{sum: sha256.New()}
		w.p.cfg.Recording = func(uuid.UUID, int, int) RecordingSink { return rec }
		ticket := w.add(terminalTicket("echo"))
		st := w.stream(t, ticket)
		synctest.Wait()
		id := w.session(t)
		assert.Equal(t, startArgs{Front: "web", Transport: "internal"}, sessionAgent(id), "StartSession gets the web front")
		assert.Equal(t, browserIP, w.trackedReq(id).IP, "the PDP decides on the browser IP")
		assert.Contains(t, w.lastStart(), browserIP.String())
		w.f.with(func(f *fakeControl) {
			assert.Empty(t, f.authCalls, "the ticket's user is the handshake: no second lookup")
		})

		st.in <- frame{data: []byte("ls\r")}
		st.in <- frame{cols: 100, rows: 30}
		synctest.Wait()
		assert.Equal(t, "win=120x40\nls\rresize=100x30\n", st.output(), "a terminal kind with the ticket window; input and resize reach it")
		assert.Contains(t, rec.events, "r:100x30", "the recording gets the window change")

		st.in <- browserClosed
		assert.Equal(t, "closed", st.closedWith(t))
		assert.Equal(t, []string{"closed"}, w.endedWith(id))
		logs := w.log.String()
		assert.NotContains(t, logs, ticket.String(), "ticket ids never reach the log")
		assert.NotContains(t, logs, "ls\r", "stream content never reaches the log")
	})
}

func TestWebFrontDenied(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		for _, resource := range []string{"denied", "missing"} { // a denied and an unknown resource read the same
			st := w.open(t, terminalTicket(resource))
			assert.Equal(t, "denied", st.closedWith(t), resource)
			assert.Equal(t, "Access denied\r\n", st.output(), resource)
			ds := w.p.denials.take()
			require.Len(t, ds, 1, resource)
			assert.Equal(t, "denied", ds[0].GetReason())
			assert.Equal(t, "terminal", ds[0].GetKind())
			assert.Equal(t, browserIP.String(), ds[0].GetClientIp())
			assert.Empty(t, ds[0].GetKeyFingerprint(), "a ticket id never goes into a denial")
		}
		assert.Zero(t, w.startCalls())
		assert.Zero(t, w.p.active.used())
	})
}

func TestWebFrontUnsupportedKind(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		st := w.open(t, terminalTicket("streamonly"))
		assert.Equal(t, "denied", st.closedWith(t))
		assert.Equal(t, "Resource streamonly does not support terminal sessions\r\n", st.output())
		assert.Zero(t, w.startCalls())
	})
}

func TestWebFrontBusy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 1)
		w.open(t, terminalTicket("blocker"))
		synctest.Wait()
		st := w.open(t, terminalTicket("blocker"))
		assert.Equal(t, "limit", st.closedWith(t))
		assert.Equal(t, "Gateway is busy, try again later\r\n", st.output())
		assert.Equal(t, 1, w.startCalls())
	})
}

func TestWebFrontShellStep(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		deny := `
@id("terminal:d")
forbid (principal, action in [Werbot::Action::"consoley.shell"], resource);`
		w := serveWeb(t, pathsPipeEnv(t, ids{}, pathsPolicies+deny))
		st := w.open(t, terminalTicket("c"))
		assert.Equal(t, "denied", st.closedWith(t))
		assert.Equal(t, "Interactive shell is not allowed on c\r\n", st.output(), "a terminal rule denies a web terminal like an SSH one")
		assert.Zero(t, w.startCalls())

		w = serveWeb(t, pathsPipeEnv(t, ids{}, pathsPolicies+consoleOn("rc")))
		st = w.open(t, terminalTicket("c"))
		synctest.Wait()
		id := w.session(t)
		assert.True(t, w.trackedReq(id).Console)
		assert.Equal(t, startArgs{Front: "web", Transport: "internal"}, sessionAgent(id))
		assert.Equal(t, "console\n", st.output(), "ServeConsole runs, Serve does not")
		st.in <- browserClosed
		assert.Equal(t, "closed", st.closedWith(t))
	})
}

func TestWebFrontRunError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		st := w.open(t, terminalTicket("fail"))
		assert.Equal(t, "error", st.closedWith(t))
		assert.Equal(t, "Login to fail failed\r\n", st.output())
		synctest.Wait()
		w.f.with(func(f *fakeControl) { require.Len(t, f.ends, 1); assert.Equal(t, "error", f.ends[0].reason) })

		st = w.open(t, terminalTicket("blocker"))
		synctest.Wait()
		id := w.session(t)
		w.p.active.Terminate(id, "terminated")
		assert.Equal(t, "closed", st.closedWith(t), "a gateway cut is no error")
		assert.Equal(t, "Session closed: terminated\r\n", st.output())
		assert.Equal(t, []string{"terminated"}, w.endedWith(id))
	})
}

// A connector panic ends the session internal_error and closes the stream with error.
func TestWebFrontConnectorPanic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		st := w.open(t, terminalTicket("panicky"))
		assert.Equal(t, "error", st.closedWith(t))
		assert.Equal(t, "Connection to panicky failed\r\n", st.output())
		synctest.Wait()
		w.f.with(func(f *fakeControl) { require.Len(t, f.ends, 1); assert.Equal(t, "internal_error", f.ends[0].reason) })
	})
}

func TestWebFrontStreamLost(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		st := w.open(t, terminalTicket("blocker"))
		synctest.Wait()
		id := w.session(t)
		st.in <- frame{err: io.ErrUnexpectedEOF}
		st.closedWith(t)
		assert.Equal(t, []string{"error"}, w.endedWith(id), "a stream lost without a close ends the session with error")
	})
}

// A connector that returns no error when its input ends still gives the lost stream the end reason "error".
func TestWebFrontStreamLostCleanExit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		st := w.open(t, terminalTicket("eof"))
		synctest.Wait()
		id := w.session(t)
		st.in <- frame{err: io.ErrUnexpectedEOF}
		st.closedWith(t)
		assert.Equal(t, []string{"error"}, w.endedWith(id))
	})
}

func TestWebFrontBrowserClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		st := w.open(t, terminalTicket("blocker"))
		synctest.Wait()
		id := w.session(t)
		st.in <- browserClosed
		assert.Equal(t, "closed", st.closedWith(t))
		assert.Equal(t, []string{"closed"}, w.endedWith(id))
	})
}

// A browser close while Open still runs cancels the open.
func TestWebFrontBrowserClosedDuringOpen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		w.f.onStart = func(ctx context.Context) { <-ctx.Done() } // StartSession waits for the server
		st := w.open(t, terminalTicket("blocker"))
		synctest.Wait()
		st.in <- browserClosed
		assert.Equal(t, "closed", st.closedWith(t))
		synctest.Wait()
		w.f.with(func(f *fakeControl) { require.Len(t, f.ends, 1); assert.Equal(t, "closed", f.ends[0].reason) })
	})
}

func TestWebFrontBadTicket(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		ticket := uuid.New() // unknown, spent, expired or of another instance: the server finds none
		w.wf.enqueue(ticket)
		synctest.Wait()
		auths, opens := w.ctl.calls()
		assert.Equal(t, 1, auths)
		assert.Zero(t, opens, "no stream is opened")
		assert.Zero(t, w.startCalls())
		assert.Contains(t, w.log.String(), "web: ticket refused")
		assert.NotContains(t, w.log.String(), ticket.String())
	})
}

func TestStreamCloseReason(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{ErrDenied, "denied"}, {accessDenied{}, "denied"}, {ErrTargetNotAllowed, "denied"}, {ErrUnsupportedType, "denied"},
		{ErrUnsupportedKind, "denied"}, {ErrApprovalDenied, "denied"}, {ErrApprovalTimeout, "denied"}, {ErrShellDenied, "denied"},
		{&CommandDeniedError{Err: ErrApprovalDenied}, "denied"}, {controlErr(io.EOF), "denied"},
		{ErrBusy, "limit"}, {ErrSpoolMax, "limit"},
		{ErrNotReady, "error"}, {ErrLoginFailed, "error"},
	} {
		assert.Equal(t, tc.want, streamCloseReason(tc.err), "%v", tc.err)
	}
}
