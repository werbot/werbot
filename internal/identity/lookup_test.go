package identity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestLookupSSHKeyAndLogin(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org1, org2 := seedOrg(t, pool, "One"), seedOrg(t, pool, "Two")
	alice := seedUser(t, pool, org1, "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")
	line, _ := newEd25519(t, "")
	k, err := svc.AddSSHKey(ctx, "k", line)
	require.NoError(t, err)

	bg := context.Background()
	want := UserRef{UserID: alice, Login: "alice", Role: "owner"}
	got, err := svc.LookupSSHKey(bg, org1, k.FingerprintSHA256)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	_, err = svc.LookupSSHKey(bg, org2, k.FingerprintSHA256)
	assert.ErrorIs(t, err, apperr.ErrNotFound)

	got, err = svc.LookupLogin(bg, org1, "alice")
	require.NoError(t, err)
	assert.Equal(t, want, got)
	_, err = svc.LookupLogin(bg, org2, "alice")
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}
