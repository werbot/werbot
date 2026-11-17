package session

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/principal"
)

func (e env) counters(t *testing.T, id uuid.UUID) [4]int64 {
	t.Helper()
	var c [4]int64
	require.NoError(t, e.pool.QueryRow(bg, `SELECT bytes_up, bytes_down, datagrams_up, datagrams_down FROM session WHERE id = $1`, id).
		Scan(&c[0], &c[1], &c[2], &c[3]))
	return c
}

func TestStartStoresKindTarget(t *testing.T) {
	e := setup(t)
	st := Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a, ClientIP: netip.MustParseAddr("10.0.0.1")}
	st.Kind, st.Target = "stream", "db.internal:5432"
	id, err := e.svc.Start(bg, st)
	require.NoError(t, err)
	var kind, target string
	require.NoError(t, e.pool.QueryRow(bg, `SELECT kind, target FROM session WHERE id = $1`, id).Scan(&kind, &target))
	assert.Equal(t, [2]string{"stream", "db.internal:5432"}, [2]string{kind, target})

	st.Kind, st.Target = "", ""
	id, err = e.svc.Start(bg, st)
	require.NoError(t, err)
	require.NoError(t, e.pool.QueryRow(bg, `SELECT kind FROM session WHERE id = $1`, id).Scan(&kind))
	assert.Equal(t, "terminal", kind, "an empty kind is a terminal session")
}

func TestUpdateCountersBatch(t *testing.T) {
	e := setup(t)
	a, b, foreign := e.open(t, &e.a, 0), e.open(t, &e.a, 0), e.open(t, &e.b, 0)
	require.NoError(t, e.svc.UpdateCounters(bg, e.org, e.a, []Counters{
		{SessionID: a, BytesUp: 10, BytesDown: 20, DatagramsUp: 1, DatagramsDown: 2},
		{SessionID: b, BytesUp: 5},
		{SessionID: foreign, BytesUp: 99},
	}))
	assert.Equal(t, [4]int64{10, 20, 1, 2}, e.counters(t, a))
	assert.Equal(t, [4]int64{5, 0, 0, 0}, e.counters(t, b))
	assert.Equal(t, [4]int64{}, e.counters(t, foreign), "a session of another instance is untouched")

	require.NoError(t, e.svc.UpdateCounters(bg, e.org, e.a, []Counters{{SessionID: a, BytesUp: 3, BytesDown: 30}}))
	assert.Equal(t, [4]int64{10, 30, 1, 2}, e.counters(t, a), "a smaller value never lowers a counter")

	require.NoError(t, e.svc.UpdateCounters(bg, uuid.New(), e.a, []Counters{{SessionID: a, BytesUp: 500}}))
	assert.Equal(t, int64(10), e.counters(t, a)[0], "another organization is ignored")
	require.NoError(t, e.svc.UpdateCounters(bg, e.org, e.a, nil))
}

func TestUpdateCountersSkipsEnded(t *testing.T) {
	e := setup(t)
	id := e.open(t, &e.a, 0)
	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed", nil))
	require.NoError(t, e.svc.UpdateCounters(bg, e.org, e.a, []Counters{{SessionID: id, BytesUp: 7}}))
	assert.Equal(t, [4]int64{}, e.counters(t, id))
}

func TestEndWithCounters(t *testing.T) {
	e := setup(t)
	id := e.open(t, &e.a, 0)
	require.NoError(t, e.svc.UpdateCounters(bg, e.org, e.a, []Counters{{SessionID: id, BytesUp: 100, BytesDown: 5}}))
	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed", &Counters{BytesUp: 50, BytesDown: 40, DatagramsUp: 3}))
	assert.Equal(t, [4]int64{100, 40, 3, 0}, e.counters(t, id), "final counters take the greater value")
}

func TestDenyWritesAudit(t *testing.T) {
	e := setup(t)
	stranger := uuid.New()
	ds := []Denial{
		{UserID: &e.user, KeyFingerprint: "SHA256:x", Resource: "web", Kind: "terminal", Reason: "no_access", ClientIP: netip.MustParseAddr("10.0.0.9")},
		{UserID: &stranger, Resource: "ghost", Kind: "stream", Target: "db:5432", Reason: "no_access"},
		{Resource: "web", Kind: "exec", Reason: "unknown_key"},
	}
	require.NoError(t, e.svc.Deny(bg, e.org, ds))
	require.NoError(t, e.svc.Deny(bg, e.org, nil))

	type row struct {
		Actor      *uuid.UUID
		Target, IP string
		Data       string
	}
	rows, err := e.pool.Query(bg, `SELECT actor_user_id, target_id, coalesce(host(ip), ''), data::text FROM audit_event
		WHERE org_id = $1 AND action = 'session.deny' AND actor_kind = 'gateway' AND target_type = 'resource' ORDER BY id`, e.org)
	require.NoError(t, err)
	var got []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.Actor, &r.Target, &r.IP, &r.Data))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 3)
	// a known name maps to the resource id, an unknown one stays; a non-member user is dropped
	want := []row{
		{&e.user, e.res.String(), "10.0.0.9", `{"kind":"terminal","target":"","reason":"no_access","key_fingerprint":"SHA256:x"}`},
		{nil, "ghost", "", `{"kind":"stream","target":"db:5432","reason":"no_access","key_fingerprint":""}`},
		{nil, e.res.String(), "", `{"kind":"exec","target":"","reason":"unknown_key","key_fingerprint":""}`},
	}
	for i := range want {
		assert.Equal(t, want[i].Actor, got[i].Actor)
		assert.Equal(t, want[i].Target, got[i].Target)
		assert.Equal(t, want[i].IP, got[i].IP)
		assert.JSONEq(t, want[i].Data, got[i].Data)
	}
}

func TestListShowsKindAndCounters(t *testing.T) {
	e := setup(t)
	id := e.open(t, &e.a, 0)
	e.exec(t, `UPDATE session SET kind = 'datagram', target = 'dns:53', bytes_up = 1, bytes_down = 2, datagrams_up = 3, datagrams_down = 4 WHERE id = $1`, id)
	s, err := e.svc.GetSession(bg, principal.Principal{UserID: e.user, OrgID: e.org, Role: principal.RoleOwner}, id)
	require.NoError(t, err)
	assert.Equal(t, [6]any{"datagram", "dns:53", int64(1), int64(2), int64(3), int64(4)},
		[6]any{s.Kind, s.Target, s.BytesUp, s.BytesDown, s.DatagramsUp, s.DatagramsDown})
}
