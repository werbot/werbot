package principal_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestRequireRole(t *testing.T) {
	_, err := principal.Require(context.Background())
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
	_, err = principal.RequireRole(context.Background(), principal.RoleOwner)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)

	p := principal.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: principal.RoleMember,
		AuthMethod: principal.AuthMethodOAuth, CLISessionID: uuid.New()}
	ctx := principal.With(context.Background(), p)
	got, ok := principal.From(ctx)
	require.True(t, ok)
	assert.Equal(t, p, got)
	_, err = principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)

	p.Role = principal.RoleAdmin
	got, err = principal.RequireRole(principal.With(context.Background(), p), principal.RoleOwner, principal.RoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, p, got)
}

func TestIsAgentManager(t *testing.T) {
	id := uuid.New()
	assert.False(t, principal.Principal{}.IsAgent())
	assert.True(t, principal.Principal{AgentID: &id}.IsAgent())
	assert.True(t, principal.Principal{RunID: &id}.IsAgent())
	assert.True(t, principal.Principal{Role: principal.RoleOwner}.Manager())
	assert.True(t, principal.Principal{Role: principal.RoleAdmin}.Manager())
	assert.False(t, principal.Principal{Role: principal.RoleMember}.Manager())
}
