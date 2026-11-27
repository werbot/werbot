package gateway

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// roEnv is a prober of a snapshot of resources with the agent_readonly checks
// of a probeCreds; the agent credential is "agent-pw", version 5.
type roEnv struct {
	h     *prober
	reg   *metrics.Registry
	creds *probeCreds
	pol   atomic.Pointer[Policy]
}

func newROEnv(rs ...Resource) *roEnv {
	e := &roEnv{reg: metrics.NewRegistry(), creds: &probeCreds{secret: Secret{Kind: "password", Value: []byte("agent-pw")}, version: 5}}
	e.setResources(rs...)
	e.h = newProber(e.pol.Load, (&dialLog{}).dial, e.reg)
	noDial := func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("no dial in tests") }
	e.h.readonly = newReadonlyChecks(e.creds.get, noDial, e.reg)
	return e
}

func (e *roEnv) setResources(rs ...Resource) { e.pol.Store(&Policy{resources: rs}) }

// pass runs one probe pass as the only instance and waits for its checks.
func (e *roEnv) pass(ctx context.Context) {
	e.h.pass(ctx, uid(0), nil)
	synctest.Wait()
}

// readonlyOf is the check result in hs, if any.
func readonlyOf(hs []*gatewayv1.ResourceHealth) (status, detail string, version int64) {
	for _, h := range hs {
		if h.GetAgentReadonlyStatus() != "" {
			return h.GetAgentReadonlyStatus(), h.GetAgentReadonlyDetail(), h.GetAgentReadonlyVersion()
		}
	}
	return "", "", 0
}

// A pending resource is checked in the next pass with the agent credential
// only, asked with the snapshot's configuration version; the result rides the
// next heartbeat with the credential's check version. The same version is not
// checked twice; a checked status waits readonlyRecheck.
func TestProbeChecksPendingImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := useRO(t, nil)
		e := newROEnv(rores(1, "pending", 77))
		e.pass(t.Context())
		require.Equal(t, []credAsk{{uid(1), 77}}, e.creds.asked(), "the snapshot's configuration version goes with the request")
		calls := f.calls()
		require.Len(t, calls, 1)
		require.True(t, calls[0].dial)
		if len(calls[0].primary.Value) != 0 || !bytes.Equal(calls[0].agent.Value, []byte("agent-pw")) {
			t.Fatal("the check must get the probe credential as agent and no primary credential (Ruling C15)")
		}
		got := e.h.take()
		require.Len(t, got, 1)
		require.True(t, got[0].GetOk(), "the probe result stays")
		status, detail, version := readonlyOf(got)
		require.Equal(t, [3]any{ReadonlyOK, "", int64(5)}, [3]any{status, detail, version})

		e.pass(t.Context())
		require.Len(t, e.creds.asked(), 2)
		require.Len(t, f.calls(), 1, "a checked version is not checked again")
		status, _, _ = readonlyOf(e.h.take())
		require.Empty(t, status)

		e.creds.set(func(c *probeCreds) { c.version = 6 })
		e.pass(t.Context())
		require.Len(t, f.calls(), 2, "a new check version is checked")
		_, _, version = readonlyOf(e.h.take())
		require.EqualValues(t, 6, version)

		e.setResources(rores(1, ReadonlyOK, 77))
		e.pass(t.Context())
		require.Len(t, e.creds.asked(), 3, "a checked status waits for the recheck")
		time.Sleep(readonlyRecheck)
		e.pass(t.Context())
		require.Len(t, f.calls(), 3, "rechecked after readonlyRecheck")
		require.Contains(t, scrape(t, e.reg), `werbot_gateway_agent_readonly_checks_total{result="ok"} 3`)
	})
}

// No check for status none (or a server without the status), a type without
// a ReadonlyChecker, or an instance that is not a prober of the resource.
func TestProbeSkipsNoneAndNonProbers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := useRO(t, nil)
		other := Resource{ID: uid(3), Name: "r3", Type: "hprobe", Config: []byte(`"10.0.0.3"`), AgentReadonly: "pending", ConfigVersion: 1}
		e := newROEnv(rores(1, "none", 1), rores(2, "", 1), other)
		e.pass(t.Context())
		require.Empty(t, e.creds.asked())

		pool := []uuid.UUID{uid(10), uid(11), uid(12), uid(13)}
		var outsider uuid.UUID
		for _, id := range pool {
			if !slices.Contains(probers(uid(4), pool, probersPerResource), id) {
				outsider = id
			}
		}
		e = newROEnv(rores(4, "pending", 1))
		e.h.pass(t.Context(), outsider, pool)
		synctest.Wait()
		require.Empty(t, e.creds.asked())
		require.Empty(t, f.calls())
	})
}

// Checks run beside the probes, at most readonlyParallel at a time: a slow
// check never delays a pass (Ruling G17).
func TestProbeReadonlySeparateSlots(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		f := useRO(t, func(ctx context.Context) (string, string) {
			select {
			case <-release:
			case <-ctx.Done():
			}
			return ReadonlyOK, ""
		})
		var rs []Resource
		for i := 1; i <= 5; i++ {
			rs = append(rs, rores(i, "pending", 1))
		}
		e := newROEnv(rs...)
		start := time.Now()
		e.h.pass(t.Context(), uid(0), nil)
		require.Zero(t, time.Since(start), "the pass does not wait for the checks")
		require.Len(t, e.h.take(), 5)
		synctest.Wait()
		require.Equal(t, readonlyParallel, f.maxRunning())
		close(release)
		synctest.Wait()
		require.Len(t, f.calls(), 5)
	})
}

// Any answer but ok or has_write is error; the detail is cut to 512 bytes
// without the credential; a check that outlives readonlyTimeout is error; a
// stopped prober reports nothing.
func TestReadonlyResultFailsClosed(t *testing.T) {
	long := "agent-pw " + strings.Repeat("INSERT on db.public.таблица; ", 40)
	for name, c := range map[string]struct {
		answer             func(ctx context.Context) (string, string)
		status             string
		after              time.Duration
		maxDetail          int
		cancelBeforeAnswer bool
	}{
		"unknown status": {answer: func(context.Context) (string, string) { return "maybe", "x" }, status: ReadonlyError, maxDetail: 1},
		"empty status":   {answer: func(context.Context) (string, string) { return "", "" }, status: ReadonlyError},
		"long detail":    {answer: func(context.Context) (string, string) { return ReadonlyHasWrite, long }, status: ReadonlyHasWrite, maxDetail: maxReadonlyDetail},
		"timeout": {answer: func(ctx context.Context) (string, string) { <-ctx.Done(); return ReadonlyError, ctx.Err().Error() },
			status: ReadonlyError, after: readonlyTimeout, maxDetail: 100},
		"stopped": {answer: func(ctx context.Context) (string, string) { <-ctx.Done(); return ReadonlyError, "stopped" }, cancelBeforeAnswer: true},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				useRO(t, c.answer)
				e := newROEnv(rores(1, "pending", 1))
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				start := time.Now()
				e.h.pass(ctx, uid(0), nil)
				if c.cancelBeforeAnswer {
					cancel()
				}
				synctest.Wait()
				if c.after > 0 {
					time.Sleep(c.after)
					synctest.Wait()
					require.Equal(t, c.after, time.Since(start))
				}
				status, detail, _ := readonlyOf(e.h.take())
				require.Equal(t, c.status, status)
				require.LessOrEqual(t, len(detail), c.maxDetail)
				require.True(t, utf8.ValidString(detail))
				if strings.Contains(detail, "agent-pw") {
					t.Fatal("the credential reached the detail")
				}
			})
		})
	}
}

// A refused or failed credential call runs no check and reports nothing; the
// next pass asks again.
func TestReadonlyCredentialErrorRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := useRO(t, nil)
		e := newROEnv(rores(1, "pending", 7))
		e.creds.set(func(c *probeCreds) {
			c.err = connect.NewError(connect.CodeFailedPrecondition, errors.New("resource changed since the snapshot"))
		})
		e.pass(t.Context())
		require.Len(t, e.creds.asked(), 1)
		require.Empty(t, f.calls())
		status, _, _ := readonlyOf(e.h.take())
		require.Empty(t, status)

		e.creds.set(func(c *probeCreds) { c.err = nil })
		e.pass(t.Context())
		require.Len(t, f.calls(), 1)
		status, _, _ = readonlyOf(e.h.take())
		require.Equal(t, ReadonlyOK, status)
	})
}

// The pipeline's checks dial through the guarded dialer and ask the control
// plane for the credential.
func TestReadonlyUsesGuardedDial(t *testing.T) {
	f := &fakeControl{connected: true}
	p := newPipeline(f, Config{DenyLocalTargets: true}, slog.New(slog.DiscardHandler))
	t.Cleanup(p.stopNow)
	_, err := p.health.readonly.dial(t.Context(), "tcp", "127.0.0.1:5432")
	require.ErrorContains(t, err, ErrTargetNotAllowed.Error())
	_, _, err = p.health.readonly.cred(t.Context(), uid(1), 1)
	require.Equal(t, connect.CodeUnimplemented, connect.CodeOf(err))
}

// A snapshot resource carries its check status and configuration version.
func TestCompilePolicyReadonlyFields(t *testing.T) {
	p, err := CompilePolicy(&gatewayv1.Snapshot{Version: "v1", Resources: []*gatewayv1.Resource{{Id: uid(1).String(), Name: "db", Type: "rocheck",
		ConfigJson: []byte(`"10.0.1.1"`), AgentReadonlyStatus: "pending", ConfigVersion: 42}}})
	require.NoError(t, err)
	r, ok := p.resourceByID(uid(1))
	require.True(t, ok)
	require.Equal(t, "pending", r.AgentReadonly)
	require.EqualValues(t, 42, r.ConfigVersion)
}
