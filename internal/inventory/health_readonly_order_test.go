package inventory_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Of two results of one check version the one that ended later wins,
// whatever order they arrive in: an older ok never replaces a newer has_write.
// A result of a new version counts whatever the time of the stored one. The
// times lie in the past, after the version began: the server clamps future
// ones to now().
func TestReportHealthAgentReadonlyNewestWins(t *testing.T) {
	h := readonlySetup(t)
	for _, q := range []string{"UPDATE credential SET updated_at = updated_at - interval '2 hours' WHERE resource_id = $1",
		"UPDATE resource SET updated_at = updated_at - interval '2 hours' WHERE id = $1"} {
		_, err := h.pool.Exec(context.Background(), q, h.res.ID)
		require.NoError(t, err)
	}
	v := h.version(t)
	base := time.Now().Add(-time.Hour) // the version began before

	h.checkAt(t, "has_write", "INSERT on db.public.t", v, base)
	h.checkAt(t, "ok", "", v, base.Add(-time.Second))
	st, detail := h.readonly(t)
	assert.Equal(t, "has_write", st, "an older ok after a newer has_write is ignored")
	assert.Equal(t, "INSERT on db.public.t", detail)

	h.checkAt(t, "has_write", "INSERT on db.public.t", v, base.Add(time.Second))
	h.checkAt(t, "ok", "", v, base.Add(time.Second/2))
	st, _ = h.readonly(t)
	assert.Equal(t, "has_write", st, "an unchanged newer result still moves the stored time")

	h.checkAt(t, "ok", "", v, base.Add(time.Minute))
	st, _ = h.readonly(t)
	assert.Equal(t, "ok", st, "a newer result wins")
	assert.Len(t, h.checkAudits(t), 2, "only changes are audited")

	_, err := h.pool.Exec(context.Background(), "UPDATE credential SET updated_at = now() + interval '1 second' WHERE resource_id = $1", h.res.ID)
	require.NoError(t, err)
	h.checkAt(t, "has_write", "UPDATE on db.public.t", h.version(t), base.Add(-time.Hour))
	st, _ = h.readonly(t)
	assert.Equal(t, "has_write", st, "the stored time of another version does not count")
}
