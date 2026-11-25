package policy

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	grantID = uuid.MustParse("019a0000-0000-7000-8000-000000000010")
	lockID  = uuid.MustParse("019a0000-0000-7000-8000-000000000011")
)

// jitSnap is one resource with no binding and the given grants and locks.
func jitSnap(grants []SnapshotGrant, locks ...SnapshotLock) Snapshot {
	return Snapshot{
		Resources: []SnapshotResource{{ID: resID, Name: "db", Type: "ssh"}},
		Grants:    grants,
		Locks:     locks,
	}
}

// at is a request of userID to resID at unix second sec.
func at(sec int64) Request {
	r := req("10.1.2.3", "DE", "")
	r.Time = time.Unix(sec, 0)
	return r
}

func compileText(t *testing.T, s Snapshot) string {
	t.Helper()
	_, text, err := Compile(s)
	require.NoError(t, err)
	return text
}

func TestCompileGrant(t *testing.T) {
	s := jitSnap([]SnapshotGrant{{ID: grantID, UserID: userID, ResourceID: resID, ExpiresUnix: 1767225600}})
	assert.Contains(t, compileText(t, s), `@id("grant:019a0000-0000-7000-8000-000000000010")
permit (principal == Werbot::User::"019a0000-0000-7000-8000-000000000001", action, resource == Werbot::Resource::"019a0000-0000-7000-8000-000000000003")
when { context.now < 1767225600 };
`)
	d := authorize(t, s, at(1767225599))
	assert.True(t, d.Allow)
	assert.Equal(t, []string{"grant:" + grantID.String()}, d.Reasons)
	d = authorize(t, s, at(1767225600))
	assert.False(t, d.Allow)
	assert.Empty(t, d.Reasons)

	zero := at(0)
	zero.Time = time.Time{}
	assert.False(t, authorize(t, s, zero).Allow, "a zero Time is now, past the grant")

	other := at(1767225599)
	other.UserID = otherID
	assert.False(t, authorize(t, s, other).Allow, "a grant is for its user only")
}

func TestCompileLocks(t *testing.T) {
	target := otherID
	cases := map[string]string{
		"user": `@id("lock:019a0000-0000-7000-8000-000000000011")
forbid (principal == Werbot::User::"019a0000-0000-7000-8000-000000000005", action, resource)
when { context.now < 1767225600 };
`,
		"team": `@id("lock:019a0000-0000-7000-8000-000000000011")
forbid (principal in Werbot::Team::"019a0000-0000-7000-8000-000000000005", action, resource)
when { context.now < 1767225600 };
`,
		"resource": `@id("lock:019a0000-0000-7000-8000-000000000011")
forbid (principal, action, resource == Werbot::Resource::"019a0000-0000-7000-8000-000000000005")
when { context.now < 1767225600 };
`,
		"cli_session": `@id("lock:019a0000-0000-7000-8000-000000000011")
forbid (principal, action, resource)
when { context.cli_session_id == "019a0000-0000-7000-8000-000000000005" && context.now < 1767225600 };
`,
	}
	for typ, want := range cases {
		text := compileText(t, jitSnap(nil, SnapshotLock{ID: lockID, TargetType: typ, TargetID: &target, ExpiresUnix: 1767225600}))
		assert.Contains(t, text, want, typ)
	}

	text := compileText(t, jitSnap(nil, SnapshotLock{ID: lockID, TargetType: "user", TargetID: &target}))
	assert.Contains(t, text, "when { context.now < 9223372036854775807 };", "a lock without expiry")

	text = compileText(t, jitSnap(nil, SnapshotLock{ID: lockID, TargetType: "mfa_device", TargetID: &target}))
	assert.NotContains(t, text, "lock:", "mfa_device")

	for _, l := range []SnapshotLock{{ID: lockID, TargetType: "bogus", TargetID: &target}, {ID: lockID, TargetType: "user"}} {
		_, _, err := Compile(jitSnap(nil, l))
		assert.Error(t, err, "an unknown target or a missing target id fails closed")
	}
}

func TestLockBeatsBindingAndGrant(t *testing.T) {
	user, team, res, other := userID, teamID, resID, otherID
	s := snap()
	s.Grants = []SnapshotGrant{{ID: grantID, UserID: userID, ResourceID: resID, ExpiresUnix: 1767225600}}
	r := at(1767225599)
	require.True(t, authorize(t, s, r).Allow)

	s.Locks = []SnapshotLock{{ID: lockID, TargetType: "user", TargetID: &user}}
	d := authorize(t, s, r)
	assert.False(t, d.Allow)
	assert.Equal(t, []string{"lock:" + lockID.String()}, d.Reasons)

	s.Locks = []SnapshotLock{{ID: lockID, TargetType: "team", TargetID: &team}}
	assert.True(t, authorize(t, s, r).Allow, "user not in the team")
	r.TeamIDs = []uuid.UUID{teamID}
	assert.False(t, authorize(t, s, r).Allow)

	s.Locks = []SnapshotLock{{ID: lockID, TargetType: "resource", TargetID: &res}}
	assert.False(t, authorize(t, s, r).Allow)
	s.Locks = []SnapshotLock{{ID: lockID, TargetType: "resource", TargetID: &other}}
	assert.True(t, authorize(t, s, r).Allow, "a lock on another resource")

	s.Locks = []SnapshotLock{{ID: lockID, TargetType: "user", TargetID: &user, ExpiresUnix: 1767225599}}
	assert.True(t, authorize(t, s, r).Allow, "an expired lock")
}

func TestLockOnGrant(t *testing.T) {
	g := grantID
	grants := []SnapshotGrant{{ID: grantID, UserID: userID, ResourceID: resID, ExpiresUnix: 5000}}

	s := jitSnap(grants, SnapshotLock{ID: lockID, TargetType: "grant", TargetID: &g})
	assert.NotContains(t, compileText(t, s), "grant:")
	assert.False(t, authorize(t, s, at(999)).Allow)

	s = jitSnap(grants, SnapshotLock{ID: lockID, TargetType: "grant", TargetID: &g, ExpiresUnix: 1000},
		SnapshotLock{ID: otherID, TargetType: "grant", TargetID: &g, ExpiresUnix: 800})
	assert.Contains(t, compileText(t, s), "when { context.now < 5000 && context.now >= 1000 };", "the latest lock wins")
	assert.NotContains(t, compileText(t, s), "lock:", "a grant lock is a condition, not a forbid")
	assert.False(t, authorize(t, s, at(999)).Allow)
	assert.True(t, authorize(t, s, at(1000)).Allow)
	assert.False(t, authorize(t, s, at(5000)).Allow)
}

func TestLockCLISession(t *testing.T) {
	sess := uuid.MustParse("019a0000-0000-7000-8000-000000000020")
	s := snap()
	s.Locks = []SnapshotLock{{ID: lockID, TargetType: "cli_session", TargetID: &sess}}
	r := at(1767225599)
	r.CLISessionID = sess
	assert.False(t, authorize(t, s, r).Allow)
	r.CLISessionID = otherID
	assert.True(t, authorize(t, s, r).Allow)
	r.CLISessionID = uuid.Nil
	assert.True(t, authorize(t, s, r).Allow)
}

func TestExpirations(t *testing.T) {
	s := Snapshot{
		Grants: []SnapshotGrant{{ExpiresUnix: 200}, {ExpiresUnix: 100}},
		Locks:  []SnapshotLock{{ExpiresUnix: 100}, {ExpiresUnix: 0}},
	}
	assert.Equal(t, []int64{100, 200}, s.Expirations())
	assert.Empty(t, Snapshot{}.Expirations())
}
