package gateway

import (
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// jitPolicy compiles a snapshot of version v with the resources "grant" and
// "blocker" (both of type blocker), the expirations exps and lockMode.
func (e *pipeEnv) jitPolicy(t *testing.T, v, policies string, exps []int64, lockMode string) *Policy {
	t.Helper()
	if _, ok := e.m["rgrant"]; !ok {
		e.m["rgrant"] = uuid.New()
	}
	s := testSnapshot(t, e.m, policies, nil, res{"rgrant", "grant", "blocker"}, res{"rblocker", "blocker", "blocker"})
	s.Version, s.Expirations, s.LockMode = v, exps, lockMode
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

// until lets u1 into "grant" before the unix second g and into "blocker"
// before b (0: always), as the server compiles a grant.
func until(g, b int64) string {
	text := fmt.Sprintf(`permit (principal == Werbot::User::"u1", action, resource == Werbot::Resource::"rgrant") when { context.now < %d };
permit (principal == Werbot::User::"u1", action, resource == Werbot::Resource::"rblocker")`, g)
	if b != 0 {
		return text + fmt.Sprintf(" when { context.now < %d };", b)
	}
	return text + ";"
}

// jitSessions opens and runs one exec session on "grant" and one on "blocker".
func (e *pipeEnv) jitSessions(t *testing.T) (granted, bound *Session) {
	t.Helper()
	granted, bound = e.open(t, "grant", KindExec), e.open(t, "blocker", KindExec)
	e.runBlocker(t, granted)
	e.runBlocker(t, bound)
	return granted, bound
}

// sleepUntil sleeps until start plus d.
func sleepUntil(start time.Time, d time.Duration) { time.Sleep(time.Until(start.Add(d))) }

// Expected: a grant expiry ends its session with access_revoked within a second
// of expires_at (between two reevalEvery passes); a session on a binding stays.
func TestExpiryCutsSessionWithinSecond(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		start := time.Now()
		exp := start.Add(150 * time.Second).Unix()
		e.f.setPolicy(e.jitPolicy(t, "v1", until(exp, 0), []int64{exp}, ""))
		startSweeper(t, e.p)
		granted, bound := e.jitSessions(t)
		sleepUntil(start, 150*time.Second-time.Millisecond)
		require.Empty(t, e.endedWith(granted.ID))
		time.Sleep(time.Second)
		require.Equal(t, []string{"access_revoked"}, e.endedWith(granted.ID))
		time.Sleep(time.Hour)
		require.Empty(t, e.endedWith(bound.ID))
	})
}

// Expected: a new snapshot re-arms the timer on its own expirations, and one
// without expirations stops it.
func TestExpiryTimerRearmsOnSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		start := time.Now()
		at := func(s int64) int64 { return start.Unix() + s }
		e.f.setPolicy(e.jitPolicy(t, "v1", until(at(70), 0), []int64{at(70)}, ""))
		startSweeper(t, e.p)
		granted, bound := e.jitSessions(t)

		sleepUntil(start, 10*time.Second)
		e.f.setPolicy(e.jitPolicy(t, "v2", until(at(15), at(45)), []int64{at(15), at(45)}, ""))
		sleepUntil(start, 15*time.Second-time.Millisecond)
		require.Empty(t, e.endedWith(granted.ID))
		time.Sleep(time.Second)
		require.Equal(t, []string{"access_revoked"}, e.endedWith(granted.ID), "re-armed on the new snapshot")

		sleepUntil(start, 20*time.Second)
		e.f.setPolicy(e.jitPolicy(t, "v3", until(at(15), at(45)), nil, ""))
		sleepUntil(start, 59*time.Second)
		require.Empty(t, e.endedWith(bound.ID), "stopped: no re-evaluation at 45 s")
		sleepUntil(start, 61*time.Second)
		require.Equal(t, []string{"access_revoked"}, e.endedWith(bound.ID), "the reevalEvery pass")
	})
}

// lose marks the tunnel down from now on, as tunnelClient.DisconnectedSince reports it.
func (f *fakeControl) lose() {
	f.with(func(f *fakeControl) { f.connected, f.downSince = false, time.Now() })
}

func (f *fakeControl) reconnect() {
	f.with(func(f *fakeControl) { f.connected, f.downSince = true, time.Time{} })
}

// Expected: in strict mode every session ends with policy_stale once the tunnel
// has been down for more than StaleAfter, and not before.
func TestStrictPolicyStale(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		start := time.Now()
		e.f.setPolicy(e.jitPolicy(t, "v1", until(start.Unix()+86400, 0), nil, "strict"))
		startSweeper(t, e.p)
		granted, bound := e.jitSessions(t)
		e.f.lose()
		sleepUntil(start, 59*time.Second)
		require.Empty(t, e.endedWith(granted.ID))
		require.Empty(t, e.endedWith(bound.ID))
		sleepUntil(start, 66*time.Second)
		require.Equal(t, []string{"policy_stale"}, e.endedWith(granted.ID))
		require.Equal(t, []string{"policy_stale"}, e.endedWith(bound.ID))
		require.Contains(t, e.log.String(), "policy is stale, sessions terminated")
	})
}

// Expected: a reconnect restarts the stale clock: two outages of 50 s keep the
// sessions; the second one still ends them past StaleAfter.
func TestStrictResetsOnReconnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		start := time.Now()
		e.f.setPolicy(e.jitPolicy(t, "v1", until(start.Unix()+86400, 0), nil, "strict"))
		startSweeper(t, e.p)
		granted, _ := e.jitSessions(t)
		e.f.lose()
		sleepUntil(start, 50*time.Second)
		e.f.reconnect()
		sleepUntil(start, 53*time.Second)
		e.f.lose()
		sleepUntil(start, 103*time.Second)
		require.Empty(t, e.endedWith(granted.ID))
		sleepUntil(start, 120*time.Second)
		require.Equal(t, []string{"policy_stale"}, e.endedWith(granted.ID))
	})
}

// Expected: best_effort keeps sessions on the last snapshot however long the tunnel is down.
func TestBestEffortKeepsSessions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.f.setPolicy(e.jitPolicy(t, "v1", until(time.Now().Unix()+86400, 0), nil, "best_effort"))
		startSweeper(t, e.p)
		granted, bound := e.jitSessions(t)
		e.f.lose()
		time.Sleep(10 * time.Minute)
		require.Empty(t, e.endedWith(granted.ID))
		require.Empty(t, e.endedWith(bound.ID))
		require.NotContains(t, e.log.String(), "policy is stale")
	})
}
