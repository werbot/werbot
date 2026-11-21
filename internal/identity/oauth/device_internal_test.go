package oauth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// TestNormalizeUserCode: any case, spaces and dashes are typed freely; the
// result is exactly the stored form XXXX-XXXX of base-20 letters.
func TestNormalizeUserCode(t *testing.T) {
	for _, in := range []string{"bcdfghjk", "BCDF-GHJK", " bcdf ghjk ", "b-c-d-f-g-h-j-k", "\tBcDf-gHjK\n"} {
		got, err := normalizeUserCode(in)
		require.NoError(t, err, in)
		assert.Equal(t, "BCDF-GHJK", got, in)
	}
	for _, in := range []string{"", "AAAA-AAAA", "BCDF-GHJKL", "BCDF-GHJ", "BCDF-GHJ1", "BCDF-GHJÖ"} {
		_, err := normalizeUserCode(in)
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, in)
	}
}

// TestDeviceConsumeOnce: two polls that both saw the code approved (two
// replicas, no lock between poll and token step) open one cli_session.
func TestDeviceConsumeOnce(t *testing.T) {
	pool, _ := dbtest.New(t)
	fx := dbtest.Seed(t, pool)
	s := &storage{pool: pool}
	newCtx := func() context.Context { return context.WithValue(context.Background(), reqInfoKey{}, &reqInfo{}) }
	require.NoError(t, s.StoreDeviceAuthorization(newCtx(), ClientCLI, "device-1", "BCDF-GHJK", time.Now().Add(time.Minute), []string{oidc.ScopeOfflineAccess}))
	_, err := pool.Exec(context.Background(), "UPDATE oauth_device_code SET status = 'approved', user_id = $1, org_id = $2", fx.MemberID, fx.OrgID)
	require.NoError(t, err)

	a, b := newCtx(), newCtx()
	for _, ctx := range []context.Context{a, b} {
		st, err := s.GetDeviceAuthorizatonState(ctx, ClientCLI, "device-1")
		require.NoError(t, err)
		require.True(t, st.Done)
	}
	_, _, _, err = s.exchangeDevice(a)
	require.NoError(t, err)
	_, _, _, err = s.exchangeDevice(b)
	assert.ErrorIs(t, err, oidc.ErrInvalidGrant())
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM cli_session").Scan(&n))
	assert.Equal(t, 1, n)
}
