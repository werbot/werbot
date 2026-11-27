package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/sshca"
	"github.com/werbot/werbot/internal/platform/version"
)

func TestOpenChecksInOrder(t *testing.T) {
	unavailable := connect.NewError(connect.CodeUnavailable, errors.New("down"))
	cases := []struct {
		name      string
		setup     func(*fakeControl)
		resource  string
		kind      Kind
		want      error
		wantAuth  int
		wantStart int
		metric    string
	}{
		{name: "no snapshot", setup: func(f *fakeControl) { f.policy = nil }, resource: "fake", want: ErrNotReady,
			metric: `werbot_gateway_sessions_rejected_total{reason="not_ready"} 1`},
		{name: "tunnel down", setup: func(f *fakeControl) { f.connected = false }, resource: "fake", want: ErrNotReady,
			metric: `werbot_gateway_sessions_rejected_total{reason="not_ready"} 1`},
		{name: "auth unavailable", setup: func(f *fakeControl) { f.authErr = unavailable }, resource: "fake", want: ErrNotReady, wantAuth: 1,
			metric: `werbot_gateway_sessions_rejected_total{reason="not_ready"} 1`},
		{name: "auth deadline", setup: func(f *fakeControl) {
			f.authErr = connect.NewError(connect.CodeDeadlineExceeded, context.DeadlineExceeded)
		},
			resource: "fake", want: ErrNotReady, wantAuth: 1},
		{name: "unknown key", setup: func(f *fakeControl) { f.authErr = connect.NewError(connect.CodeNotFound, errors.New("no key")) },
			resource: "fake", want: ErrDenied, wantAuth: 1},
		{name: "forbidden resource", resource: "denied", want: ErrDenied, wantAuth: 1},
		{name: "missing resource", resource: "missing", want: ErrDenied, wantAuth: 1}, // reads the same as "forbidden resource"
		{name: "no connector", resource: "nope", want: ErrUnsupportedType, wantAuth: 1},
		{name: "kind not served", resource: "streamonly", want: ErrUnsupportedKind, wantAuth: 1},
		{name: "start unavailable", setup: func(f *fakeControl) { f.startErr = unavailable }, resource: "fake", want: ErrNotReady, wantAuth: 1, wantStart: 1},
		{name: "start denied", setup: func(f *fakeControl) { f.startErr = connect.NewError(connect.CodePermissionDenied, errors.New("no")) },
			resource: "fake", want: ErrDenied, wantAuth: 1, wantStart: 1},
		{name: "no credential", setup: func(f *fakeControl) { f.startErr = connect.NewError(connect.CodeNotFound, errors.New("no credential")) },
			resource: "fake", want: ErrDenied, wantAuth: 1, wantStart: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newPipeEnv(t, 10)
			if tc.setup != nil {
				e.f.with(tc.setup)
			}
			kind := tc.kind
			if kind == "" {
				kind = KindTerminal
			}
			_, err := e.p.Open(t.Context(), e.req(tc.resource, kind))
			require.ErrorIs(t, err, tc.want)
			e.f.with(func(f *fakeControl) {
				require.Len(t, f.authCalls, tc.wantAuth)
				require.Equal(t, tc.wantStart, f.startCalls)
				require.Empty(t, f.recs)
			})
			if tc.metric != "" {
				require.Contains(t, e.metrics(t), tc.metric)
			}
			require.Zero(t, e.p.active.used(), "a failed Open frees its slot")
		})
	}

	t.Run("busy", func(t *testing.T) {
		e := newPipeEnv(t, 1)
		e.open(t, "fake", KindTerminal)
		_, err := e.p.Open(t.Context(), e.req("fake", KindTerminal))
		require.ErrorIs(t, err, ErrBusy)
		e.f.with(func(f *fakeControl) { require.Len(t, f.authCalls, 1, "AuthenticateUser is not called when busy") })
		require.Contains(t, e.metrics(t), `werbot_gateway_sessions_rejected_total{reason="busy"} 1`)
	})

	t.Run("success", func(t *testing.T) {
		e := newPipeEnv(t, 10)
		s := e.open(t, "fake", KindTerminal)
		require.NotEqual(t, [16]byte{}, s.ID)
		require.Equal(t, KindTerminal, s.Kind)
		require.Equal(t, "ssh", s.Front)
		require.Equal(t, "alice", s.User.Login)
		require.Equal(t, e.m["rfake"], s.Resource.ID)
		require.Equal(t, "fake", s.Resource.Type)
		require.Equal(t, "s3cret-value", string(s.Secret.Value))
		require.Equal(t, "192.0.2.7", s.ClientIP.String())
		require.NotNil(t, s.Recording)
		e.f.with(func(f *fakeControl) {
			require.Equal(t, []string{"SHA256:key 192.0.2.7"}, f.authCalls)
			require.Equal(t, []string{fmt.Sprintf("%s 120 40", s.ID)}, f.recs)
		})
		require.True(t, e.inHeartbeat(s.ID), "an opened session is reported before Run")
	})
}

func TestRegisterConnector(t *testing.T) {
	require.PanicsWithValue(t, `gateway: connector "fake" already registered`, func() { RegisterConnector(fakeConn{}) })
	require.PanicsWithValue(t, `gateway: connector "nokinds" has no kinds`, func() { RegisterConnector(funcConn{typ: "nokinds"}) })
	_, ok := LookupConnector("nokinds")
	require.False(t, ok)
	c, ok := LookupConnector("fake")
	require.True(t, ok)
	require.Equal(t, "fake", c.Type())
	_, ok = LookupConnector("nope")
	require.False(t, ok)
	require.Equal(t, []string{"blocker", "consoley", "echo", "eof", "execy", "failing", "fake", "masky", "panicky", "plainstream", "preconn", "querylogger", "quiet", "rfwd", "rfwdbare", "spy", "streamonly"}, connectors())
}

func TestPinKeyCallsServer(t *testing.T) {
	e := newPipeEnv(t, 10)
	s := e.open(t, "fake", KindTerminal)
	require.NoError(t, s.PinKey("ssh-ed25519", []byte("key-bytes")))
	e.f.with(func(f *fakeControl) {
		require.Equal(t, []string{e.m["rfake"].String() + " ssh-ed25519 key-bytes"}, f.pins)
		f.pinErr = ErrHostKeyMismatch
	})
	require.ErrorIs(t, s.PinKey("ssh-ed25519", []byte("other")), ErrHostKeyMismatch)
}

func TestReadyAndHeartbeat(t *testing.T) {
	e := newPipeEnv(t, 1)
	ok, reason := e.p.ready()
	require.True(t, ok)
	require.Empty(t, reason)

	e.f.with(func(f *fakeControl) { f.connected = false })
	ok, reason = e.p.ready()
	require.False(t, ok)
	require.Equal(t, "not_ready", reason)

	e.f.with(func(f *fakeControl) { f.connected = true })
	s := e.open(t, "fake", KindTerminal)
	ok, reason = e.p.ready()
	require.False(t, ok)
	require.Equal(t, "busy", reason)

	hb := e.p.heartbeat()
	require.Equal(t, version.Version, hb.GetVersion())
	require.Equal(t, connectors(), hb.GetConnectors())
	require.Equal(t, []string{s.ID.String()}, hb.GetActiveSessionIds())
}

func TestActiveSessionsIDsCapped(t *testing.T) {
	log := &syncBuffer{}
	a := newActiveSessions(metrics.NewRegistry().Gauge("g", "g"), slog.New(slog.NewTextHandler(log, nil)))
	sp := newTestSpool(t, &fakeControl{upErr: connect.NewError(connect.CodeUnavailable, errors.New("down"))})
	opened := map[string]bool{}
	for range maxSessionIDs - 2 {
		id := uuid.New()
		a.add(id, &tracked{})
		opened[id.String()] = true
	}
	for range 5 {
		id := uuid.New()
		a.add(id, &tracked{})
		require.NoError(t, sp.End(id, "closed", nil, nil))
		a.done(id)
	}
	ids := heartbeatSessionIDs(a, sp)
	require.Len(t, ids, maxSessionIDs)
	n := 0
	for _, id := range ids {
		if opened[id] {
			n++
		}
	}
	require.Equal(t, len(opened), n, "every opened session is listed")
	require.Contains(t, log.String(), "omitted=3")

	a.release() // no slot taken
	require.Zero(t, a.used())
	require.Contains(t, log.String(), "slot released twice")
}

// Expected: the CLI session of the user's certificate reaches the PDP on Open
// and on every re-evaluation: a cli_session lock ends and refuses only that
// session; key authentication has no CLI session.
func TestCLISessionLock(t *testing.T) {
	e := newPipeEnv(t, 10)
	startSweeper(t, e.p)
	locked, other := uuid.New(), uuid.New()
	req := func(cli uuid.UUID) OpenRequest {
		r := e.req("blocker", KindExec)
		if cli != uuid.Nil {
			r.Identity = Identity{Method: "ssh-cert", Value: "SHA256:cert", Cert: &sshca.Identity{UserID: e.m["u1"], CLISessionID: cli}}
		}
		return r
	}
	open := func(cli uuid.UUID) *Session {
		s, err := e.p.Open(t.Context(), req(cli))
		require.NoError(t, err)
		e.runBlocker(t, s)
		return s
	}
	a, b, key := open(locked), open(other), open(uuid.Nil)
	e.f.setPolicy(compileV(t, e.m, "locked", fmt.Sprintf(`permit (principal == Werbot::User::"u1", action, resource);
forbid (principal, action, resource) when { context.cli_session_id == %q && context.now < 9223372036854775807 };`, locked.String()),
		res{"rblocker", "blocker", "blocker"}))
	e.ended(t, a.ID, "access_revoked")
	e.stillRunning(t, b.ID)
	e.stillRunning(t, key.ID)
	_, err := e.p.Open(t.Context(), req(locked))
	require.ErrorIs(t, err, ErrDenied)
	open(other)
}
