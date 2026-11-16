package inventory_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// token creates a join token of gatewayID with the default TTL.
func (e gwEnv) token(t *testing.T, gatewayID uuid.UUID, maxUses int) string {
	t.Helper()
	_, tok, _, err := e.gw.CreateJoinToken(e.ctx, gatewayID, 0, maxUses)
	require.NoError(t, err)
	return tok
}

// secret is the part of a join token whose hash is stored.
func secret(tok string) string {
	s, _, _ := strings.Cut(strings.TrimPrefix(tok, "wjt_"), ".")
	return s
}

func TestCreateJoinToken(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	id, tok, exp, err := e.gw.CreateJoinToken(e.ctx, g.ID, 0, 0)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), exp, time.Minute)
	assert.Regexp(t, `^wjt_[A-Za-z0-9_-]{43}\.[0-9a-f]{64}$`, tok)
	assert.True(t, strings.HasSuffix(tok, "."+e.ca.Fingerprint()))

	var hash []byte
	var maxUses int
	var createdBy *uuid.UUID
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT token_hash, max_uses, created_by FROM gateway_join_token WHERE id = $1", id).Scan(&hash, &maxUses, &createdBy))
	sum := sha256.Sum256([]byte(secret(tok)))
	assert.Equal(t, sum[:], hash)
	assert.Equal(t, 10, maxUses)
	assert.Equal(t, &e.owner, createdBy)

	for _, tc := range []struct {
		ttl  time.Duration
		uses int
		ok   bool
	}{
		{59 * time.Minute, 1, false}, {8761 * time.Hour, 1, false}, {-time.Hour, 1, false},
		{time.Hour, 1001, false}, {time.Hour, -1, false},
		{time.Hour, 1, true}, {8760 * time.Hour, 1000, true},
	} {
		_, _, _, err := e.gw.CreateJoinToken(e.ctx, g.ID, tc.ttl, tc.uses)
		if tc.ok {
			assert.NoError(t, err, "%v %d", tc.ttl, tc.uses)
		} else {
			assert.ErrorIs(t, err, apperr.ErrInvalidArgument, "%v %d", tc.ttl, tc.uses)
		}
	}

	_, orgB := e.otherOrg(t)
	_, _, _, err = e.gw.CreateJoinToken(e.ctx, dbtest.Gateway(t, e.pool, orgB, "b"), 0, 0)
	require.ErrorIs(t, err, apperr.ErrNotFound)
	assert.NotContains(t, err.Error(), secret(tok))

	var raw []byte
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT data FROM audit_event WHERE action = 'gateway.join_token.create' AND target_id = $1 AND actor_user_id = $2",
		id.String(), e.owner).Scan(&raw))
	var data map[string]any
	require.NoError(t, json.Unmarshal(raw, &data))
	assert.Equal(t, g.ID.String(), data["gateway_id"])
	assert.Equal(t, id.String(), data["token_id"])
	assert.EqualValues(t, 10, data["max_uses"])
	assert.NotEmpty(t, data["expires_at"])
	var all string
	require.NoError(t, e.pool.QueryRow(context.Background(), "SELECT string_agg(data::text, '') FROM audit_event").Scan(&all))
	assert.NotContains(t, all, secret(tok))
}

func TestListJoinTokens(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	id1, tok1, exp1, err := e.gw.CreateJoinToken(e.ctx, g.ID, 2*time.Hour, 3)
	require.NoError(t, err)
	id2, _, _, err := e.gw.CreateJoinToken(e.ctx, g.ID, 0, 0)
	require.NoError(t, err)
	require.NoError(t, e.gw.RevokeJoinToken(e.ctx, id2))
	e.enroll(t, tok1, "gw-a")

	list, err := e.gw.ListJoinTokens(e.ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, id1, list[0].ID)
	assert.Equal(t, 3, list[0].MaxUses)
	assert.Equal(t, 1, list[0].Uses)
	assert.WithinDuration(t, exp1, list[0].ExpiresAt, time.Second)
	assert.Nil(t, list[0].RevokedAt)
	assert.Equal(t, &e.owner, list[0].CreatedBy)
	assert.False(t, list[0].CreatedAt.IsZero())
	assert.Equal(t, id2, list[1].ID)
	assert.NotNil(t, list[1].RevokedAt)
	assert.NotContains(t, fmt.Sprintf("%+v", list), secret(tok1), "no secret in the listing")
}

func TestCreateSystemJoinToken(t *testing.T) {
	e := gwSetup(t)
	r := e.create(t, "web", nil)
	toks := make([]string, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Go(func() { toks[i], errs[i] = e.gw.CreateSystemJoinToken(context.Background(), e.org) })
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	_, err := e.gw.CreateSystemJoinToken(context.Background(), e.org)
	require.NoError(t, err)

	var local uuid.UUID
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT id FROM gateway WHERE org_id = $1 AND name = 'local'", e.org).Scan(&local))
	assert.Equal(t, 1, e.count(t, "SELECT count(*) FROM gateway WHERE org_id = $1", e.org))
	got, err := e.svc.Get(e.ctx, e.org, r.ID)
	require.NoError(t, err)
	assert.Equal(t, &local, got.GatewayID, "the first pool takes unassigned resources")

	rows, err := e.pool.Query(context.Background(),
		"SELECT max_uses, expires_at, created_by FROM gateway_join_token WHERE gateway_id = $1", local)
	require.NoError(t, err)
	n := 0
	for rows.Next() {
		var maxUses int
		var exp time.Time
		var createdBy *uuid.UUID
		require.NoError(t, rows.Scan(&maxUses, &exp, &createdBy))
		assert.Equal(t, 1, maxUses)
		assert.WithinDuration(t, time.Now().Add(time.Hour), exp, time.Minute)
		assert.Nil(t, createdBy)
		n++
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, 3, n)
	assert.Equal(t, 1, e.count(t,
		"SELECT count(*) FROM audit_event WHERE action = 'gateway.create' AND actor_kind = 'system' AND actor_user_id IS NULL"))
	assert.Equal(t, 3, e.count(t,
		"SELECT count(*) FROM audit_event WHERE action = 'gateway.join_token.create' AND actor_kind = 'system'"))

	e.enroll(t, toks[0], "local-1")
	_, err = e.gw.Enroll(context.Background(), enrollReq(toks[0], csr(t), "local-2"))
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied, "system tokens are single-use")
}
