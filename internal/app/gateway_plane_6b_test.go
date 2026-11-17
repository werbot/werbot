package app

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// One heartbeat carries counters, a denial and a probe result: the server
// stores them in session, audit_event and resource_health; EndSession stores
// the final counters.
func TestHeartbeatCountersDenialsHealth(t *testing.T) {
	s := startStack(t)
	res := s.addResource(t, "web", "echo")
	_, instance, _ := instanceIdentity(t, s.gwData)
	client := instanceClient(t, s.gwData)
	ctx := context.Background()
	started, err := client.StartSession(ctx, connect.NewRequest(&gatewayv1.StartSessionRequest{
		UserId: s.alice, ResourceId: res.String(), ClientIp: "10.0.0.1", Kind: "stream", Target: "web:80"}))
	require.NoError(t, err)
	sid := started.Msg.GetSessionId()

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	st := client.Connect(sctx)
	at := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{
		Version: "test", ActiveSessionIds: []string{sid},
		Counters: []*gatewayv1.SessionCounters{{SessionId: sid, BytesUp: 10, BytesDown: 20, DatagramsUp: 1, DatagramsDown: 2}},
		Denials: []*gatewayv1.Denial{{UserId: s.alice, Resource: "web", Kind: "stream", Target: "web:5432", ClientIp: "10.0.0.2",
			Reason: "target_not_allowed", At: timestamppb.New(at)}},
		Health: []*gatewayv1.ResourceHealth{{ResourceId: res.String(), Error: "connection refused", CheckedAt: timestamppb.New(at)}},
	}}}))
	// The pool list follows the first snapshot and names this (online) instance.
	for {
		m, err := st.Receive()
		require.NoError(t, err)
		if p := m.GetPoolInstances(); p != nil {
			assert.Contains(t, p.GetIds(), instance.String())
			break
		}
	}

	var counters [4]int64
	require.Eventually(t, func() bool {
		require.NoError(t, s.pool.QueryRow(ctx, `SELECT bytes_up, bytes_down, datagrams_up, datagrams_down FROM session WHERE id = $1`, sid).
			Scan(&counters[0], &counters[1], &counters[2], &counters[3]))
		return counters[0] == 10
	}, 5*time.Second, 20*time.Millisecond)
	assert.Equal(t, [4]int64{10, 20, 1, 2}, counters)

	var target, data string
	require.Eventually(t, func() bool {
		return s.pool.QueryRow(ctx, `SELECT target_id, data::text FROM audit_event WHERE action = 'session.deny'`).Scan(&target, &data) == nil
	}, 5*time.Second, 20*time.Millisecond)
	assert.Equal(t, res.String(), target)
	assert.JSONEq(t, `{"kind":"stream","target":"web:5432","reason":"target_not_allowed","key_fingerprint":"","at":"`+at.Format(time.RFC3339)+`"}`, data)

	var ok bool
	var errText string
	require.Eventually(t, func() bool {
		return s.pool.QueryRow(ctx, `SELECT ok, error FROM resource_health WHERE resource_id = $1 AND gateway_instance_id = $2`, res, instance).
			Scan(&ok, &errText) == nil
	}, 5*time.Second, 20*time.Millisecond)
	assert.False(t, ok)
	assert.Equal(t, "connection refused", errText)

	_, err = client.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{SessionId: sid, Reason: "idle_timeout",
		Counters: &gatewayv1.SessionCounters{SessionId: sid, BytesUp: 15, BytesDown: 5, DatagramsUp: 3, DatagramsDown: 4}}))
	require.NoError(t, err)
	var reason string
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT end_reason, bytes_up, bytes_down, datagrams_up, datagrams_down FROM session WHERE id = $1`, sid).
		Scan(&reason, &counters[0], &counters[1], &counters[2], &counters[3]))
	assert.Equal(t, "idle_timeout", reason)
	assert.Equal(t, [4]int64{15, 20, 3, 4}, counters, "final counters never lower the stored ones")
}

func TestSnapshotResourceTimeouts(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, owner := s.org(t, "owner")
	gw, ids := s.gatewayPool(ctx, t, owner)
	_, err := s.pool.Exec(ctx, `UPDATE resource SET idle_timeout = '10 minutes', max_duration = '2 hours' WHERE id = $1`, ids[0])
	require.NoError(t, err)

	snap, err := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots.Build(context.Background(), org, gw)
	require.NoError(t, err)
	require.Len(t, snap.GetResources(), 3)
	for _, r := range snap.GetResources() {
		if r.GetId() == ids[0] {
			assert.Equal(t, 10*time.Minute, r.GetIdleTimeout().AsDuration())
			assert.Equal(t, 2*time.Hour, r.GetMaxDuration().AsDuration())
			continue
		}
		assert.Nil(t, r.GetIdleTimeout(), "unset: the gateway default")
		assert.Nil(t, r.GetMaxDuration())
	}
}

// Removing a user from a team or the organization changes the snapshot (and
// so its version): the gateway re-evaluates that user's sessions. Unchanged
// data keeps it; the deny-all snapshot carries the members too.
func TestSnapshotMembersVersion(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, owner := s.org(t, "owner")
	gw, _ := s.gatewayPool(ctx, t, owner)
	_, otherOrg, _ := s.org(t, "owner")
	require.NotEqual(t, org, otherOrg)
	bob := dbtest.User(t, s.pool, org, "member")
	ops, err := s.d.Identity.CreateTeam(ctx, "ops")
	require.NoError(t, err)
	dev, err := s.d.Identity.CreateTeam(ctx, "dev")
	require.NoError(t, err)
	for _, team := range []uuid.UUID{ops.ID, dev.ID} {
		require.NoError(t, s.d.Identity.AddTeamMember(ctx, team, owner))
	}
	build := func() *gatewayv1.Snapshot {
		snap, err := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots.Build(context.Background(), org, gw)
		require.NoError(t, err)
		return snap
	}
	member := func(snap *gatewayv1.Snapshot, user uuid.UUID) *gatewayv1.Member {
		for _, m := range snap.GetMembers() {
			if m.GetUserId() == user.String() {
				return m
			}
		}
		return nil
	}
	teams := []string{ops.ID.String(), dev.ID.String()}
	slices.Sort(teams)

	before := build()
	assert.True(t, proto.Equal(before, build()), "unchanged data, same snapshot and version")
	require.Len(t, before.GetMembers(), 2, "members of the pool's organization only")
	assert.True(t, slices.IsSortedFunc(before.GetMembers(), func(a, b *gatewayv1.Member) int { return strings.Compare(a.GetUserId(), b.GetUserId()) }))
	assert.Equal(t, teams, member(before, owner).GetTeamIds())
	assert.Empty(t, member(before, bob).GetTeamIds())

	require.NoError(t, s.d.Identity.RemoveTeamMember(ctx, ops.ID, owner))
	after := build()
	assert.False(t, proto.Equal(before, after), "the snapshot version changes")
	assert.Equal(t, []string{dev.ID.String()}, member(after, owner).GetTeamIds())

	require.NoError(t, s.d.Identity.RemoveMember(ctx, bob))
	removed := build()
	assert.False(t, proto.Equal(after, removed), "an org removal changes the version")
	assert.Nil(t, member(removed, bob), "a removed user is absent from members")

	// A policy that does not compile still carries the members.
	_, err = s.pool.Exec(ctx, `ALTER TABLE access_binding DROP CONSTRAINT access_binding_subject_type_check`)
	require.NoError(t, err)
	_, err = s.pool.Exec(ctx, `UPDATE access_binding SET subject_type = 'robot' WHERE org_id = $1`, org)
	require.NoError(t, err)
	denyAll := build()
	assert.Empty(t, denyAll.GetCedarPolicies())
	assert.True(t, proto.Equal(&gatewayv1.Snapshot{Members: removed.GetMembers()}, &gatewayv1.Snapshot{Members: denyAll.GetMembers()}))
}
