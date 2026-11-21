package policy

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestTestCommandPaths(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	e.bind(e.org, e.owner, r)
	rule, err := e.svc.CreatePathRule(e.ctx, e.org, appOnly(r))
	require.NoError(t, err)
	id := rule.ID.String()
	test := func(command, cwd string) CommandResult {
		t.Helper()
		got, err := e.svc.TestCommand(e.ctx, e.org, e.owner, r, command, cwd, ip10, time.Time{})
		require.NoError(t, err, command)
		return got
	}

	got := test("cat /etc/hosts", "")
	assert.False(t, got.Allow)
	assert.Equal(t, []string{"/etc/hosts"}, got.Paths)
	assert.Equal(t, []string{id}, got.PathOutside)
	assert.Empty(t, got.PathHits)
	assert.Equal(t, []string{"path:" + id}, got.Reasons)
	assert.Equal(t, PathOutsideMessage, got.Message)

	got = test("cat app/a", "")
	assert.True(t, got.Allow)
	assert.Equal(t, []string{"~/app/a"}, got.Paths)
	assert.Empty(t, got.PathOutside)

	assert.True(t, test("cat a", "~/app").Allow, "relative path in an allowed cwd")

	got = test("cat x", "/srv")
	assert.False(t, got.Allow)
	assert.Equal(t, []string{"/srv/x"}, got.Paths)

	got = test("cat $F", "")
	assert.Contains(t, got.Classes, "opaque")
	assert.Empty(t, got.Paths)
	assert.Equal(t, []string{id}, got.PathOutside, "unresolved path is outside every only rule")
	assert.False(t, got.Allow)

	// path_hits names the rules of the whole organization; the decision only
	// the rules of the subject.
	member := dbtest.User(t, e.pool, e.org, "member")
	deny := appOnly(r)
	deny.Name, deny.Subjects, deny.Effect, deny.Paths = "no-secret", Subjects{Users: []uuid.UUID{member}}, "deny", []string{"~/app/secret/**"}
	denyRule, err := e.svc.CreatePathRule(e.ctx, e.org, deny)
	require.NoError(t, err)
	got = test("cat app/secret/k", "")
	assert.Equal(t, []string{denyRule.ID.String()}, got.PathHits)
	assert.True(t, got.Allow)
}

func TestTestCommandRejectsCwd(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	for _, cwd := range []string{"x/../y", "app", "/srv/", "/a/./b", "~user", "/a\x01b", "/a\nb", "/" + strings.Repeat("a", 1024)} {
		_, err := e.svc.TestCommand(e.ctx, e.org, e.owner, r, "ls", cwd, ip10, time.Time{})
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, cwd)
		assert.Contains(t, err.Error(), "invalid cwd", cwd)
	}
	for _, cwd := range []string{"", "~", "~/app", "/", "/" + strings.Repeat("a", 1023)} {
		_, err := e.svc.TestCommand(e.ctx, e.org, e.owner, r, "ls", cwd, ip10, time.Time{})
		assert.NoError(t, err, cwd)
	}
}
