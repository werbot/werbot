package gateway

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// readText reads len(want) bytes from client within a minute of bubble time:
// a missing text fails the test instead of hanging it.
func readText(t *testing.T, client net.Conn, want string) {
	t.Helper()
	require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Minute)))
	got := make([]byte, len(want))
	_, err := io.ReadFull(client, got)
	require.NoError(t, err)
	require.Equal(t, want, string(got))
}

// runTerminal runs s on the blocker connector with a terminal on a pipe; the
// returned conn is the user's side.
func (e *pipeEnv) runTerminal(t *testing.T, s *Session) (net.Conn, <-chan struct{}) {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	s.Terminal = &Terminal{ReadWriter: server}
	return client, e.runBlocker(t, s)
}

func TestTerminateCommand(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		s := e.open(t, "blocker", KindTerminal)
		client, done := e.runTerminal(t, s)
		synctest.Wait()
		e.p.active.Terminate(s.ID, "terminated")
		readText(t, client, "Session closed: terminated\r\n")
		<-done
		require.Equal(t, []string{"terminated"}, e.endedWith(s.ID))
		require.Equal(t, -1, exitStatus(s, 0), "a terminated session has no exit status")

		e.p.active.Terminate(s.ID, "terminated") // a repeat: no-op
		e.p.active.Terminate(uuid.New(), "terminated")
		require.Equal(t, []string{"terminated"}, e.endedWith(s.ID))
		require.Zero(t, e.p.active.used())
	})
}

// terminatingControl terminates every session it starts before Open tracks it.
type terminatingControl struct {
	*fakeControl
	a *activeSessions
}

func (c terminatingControl) StartSession(ctx context.Context, userID, resourceID uuid.UUID, ip netip.Addr, kind Kind, target, command string, classes, paths []string, args startArgs) (uuid.UUID, Secret, error) {
	id, sec, err := c.fakeControl.StartSession(ctx, userID, resourceID, ip, kind, target, command, classes, paths, args)
	c.a.Terminate(id, "terminated")
	return id, sec, err
}

func TestTerminateBeforeAdd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.p.ctl = terminatingControl{e.f, e.p.active}
		s := e.open(t, "blocker", KindTerminal)
		client, done := e.runTerminal(t, s)
		readText(t, client, "Session closed: terminated\r\n")
		<-done
		require.Equal(t, []string{"terminated"}, e.endedWith(s.ID))
	})

	synctest.Test(t, func(t *testing.T) {
		a := newActiveSessions(metrics.NewRegistry().Gauge("g", "g"), slog.New(slog.DiscardHandler))
		late, kept := uuid.New(), uuid.New()
		a.Terminate(late, "terminated")
		time.Sleep(time.Minute)
		a.Terminate(kept, "terminated")
		time.Sleep(59 * time.Second)
		lt, kt := &tracked{}, &tracked{}
		a.add(late, lt)
		a.add(kept, kt)
		require.Nil(t, lt.cause, "a termination is kept for a minute only")
		require.Equal(t, endCause("terminated"), kt.cause)

		first := uuid.New()
		a.Terminate(first, "terminated")
		time.Sleep(time.Second)
		for range earlyMax {
			a.Terminate(uuid.New(), "terminated")
		}
		require.Len(t, a.early, earlyMax)
		ft := &tracked{}
		a.add(first, ft)
		require.Nil(t, ft.cause, "the oldest termination makes room")
	})
}
