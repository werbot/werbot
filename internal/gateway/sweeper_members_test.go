package gateway

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// teamPolicies grant u1 "fake" through team t1 and "blocker" directly.
const teamPolicies = `
permit (principal in Werbot::Team::"t1", action, resource == Werbot::Resource::"rfake");
permit (principal == Werbot::User::"u1", action, resource == Werbot::Resource::"rblocker");`

// teamEnv runs a sweeper on a pipeline whose user u1 is in team t1; it returns
// the sessions on "fake" (team grant) and "blocker" (user grant).
func teamEnv(t *testing.T) (e *pipeEnv, team, user *Session) {
	t.Helper()
	e = newPipeEnv(t, 10)
	e.m["t1"], e.m["u2"] = uuid.New(), uuid.New()
	e.f.with(func(f *fakeControl) { f.user.TeamIDs = []uuid.UUID{e.m["t1"]} })
	e.f.setPolicy(e.memberSnapshot(t, "v1", e.member("u1", "t1")))
	startSweeper(t, e.p)
	team, user = e.open(t, "fake", KindExec), e.open(t, "blocker", KindExec)
	e.runBlocker(t, team)
	e.runBlocker(t, user)
	return e, team, user
}

// member lists user (a short name) with teams.
func (e *pipeEnv) member(user string, teams ...string) *gatewayv1.Member {
	m := &gatewayv1.Member{UserId: e.m[user].String()}
	for _, t := range teams {
		m.TeamIds = append(m.TeamIds, e.m[t].String())
	}
	return m
}

func (e *pipeEnv) memberSnapshot(t *testing.T, v string, members ...*gatewayv1.Member) *Policy {
	t.Helper()
	return e.plainSnapshot(t, teamPolicies, v, func(s *gatewayv1.Snapshot) { s.Members = members })
}

// ended waits up to a second for the delivered ends of id.
func (e *pipeEnv) ended(t *testing.T, id uuid.UUID, reasons ...string) {
	t.Helper()
	require.Eventually(t, func() bool { return len(e.f.endsOf(id)) == len(reasons) }, time.Second, 5*time.Millisecond)
	require.Equal(t, reasons, e.f.endsOf(id))
}

// stillRunning checks that id has no end after the sweeper had time to act.
func (e *pipeEnv) stillRunning(t *testing.T, id uuid.UUID) {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	require.Empty(t, e.f.endsOf(id))
}

func TestRevokeOnTeamRemoval(t *testing.T) {
	e, team, user := teamEnv(t)
	e.f.setPolicy(e.memberSnapshot(t, "v2", e.member("u1", "t1"), e.member("u2")))
	e.stillRunning(t, team.ID)
	e.f.setPolicy(e.memberSnapshot(t, "v3", e.member("u1"), e.member("u2", "t1")))
	e.ended(t, team.ID, "access_revoked")
	e.stillRunning(t, user.ID)
}

func TestRevokeOnOrgRemoval(t *testing.T) {
	e, team, user := teamEnv(t)
	e.f.setPolicy(e.memberSnapshot(t, "v2", e.member("u2", "t1")))
	e.ended(t, user.ID, "access_revoked")
	e.ended(t, team.ID, "access_revoked")
}

// An older server sends no members: the teams from AuthenticateUser stay.
func TestEmptyMembersKeepsSessions(t *testing.T) {
	e, team, user := teamEnv(t)
	e.f.setPolicy(e.memberSnapshot(t, "v2"))
	e.stillRunning(t, team.ID)
	e.stillRunning(t, user.ID)
}

// Re-evaluation reads the resource again: a narrowed endpoint revokes, new limits apply.
func TestReevaluationReadsResource(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPlainEnv(t)
		startSweeper(t, e.p)
		moved, _, _ := e.runStream(t, "10.0.0.5:5432")
		byName, _, _ := e.runStream(t, "pg:5432")
		e.f.setPolicy(e.plainSnapshot(t, plainPolicies, "v2", func(s *gatewayv1.Snapshot) {
			s.Resources[2].ConfigJson = []byte(`"10.0.0.6"`)
			withLimits(0, 2*time.Minute)(s)
		}))
		require.Equal(t, []string{"access_revoked"}, e.endedWith(moved.ID), "the host left the endpoint")
		require.Empty(t, e.endedWith(byName.ID))
		time.Sleep(2*time.Minute + sweepEvery)
		require.Equal(t, []string{"max_duration"}, e.endedWith(byName.ID), "the new max duration applies")

		gone, _, _ := e.runStream(t, "pg:5432")
		e.f.setPolicy(e.plainSnapshot(t, plainPolicies, "v3", func(s *gatewayv1.Snapshot) { s.Resources = s.Resources[:2] }))
		require.Equal(t, []string{"access_revoked"}, e.endedWith(gone.ID), "the resource was deleted")
	})
}
