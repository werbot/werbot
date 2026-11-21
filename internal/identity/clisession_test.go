package identity

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func sessionIDs(list []CLISession) []uuid.UUID {
	ids := make([]uuid.UUID, len(list))
	for i, c := range list {
		ids[i] = c.ID
	}
	return ids
}

func TestListCLISessions(t *testing.T) {
	s := newCLIStand(t)
	ctx := context.Background()
	org, other := seedOrg(t, s.pool, "Acme"), seedOrg(t, s.pool, "Other")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleMember)
	bob := seedUser(t, s.pool, org, "bob", principal.RoleMember)
	seedUser(t, s.pool, org, "adm", principal.RoleAdmin)
	eve := seedUser(t, s.pool, other, "eve", principal.RoleMember)
	t1, t2 := s.cliLogin(t, org, alice), s.cliLogin(t, org, alice)
	revoked, expired := s.cliLogin(t, org, alice), s.cliLogin(t, org, alice)
	s.cliLogin(t, org, bob)
	s.cliLogin(t, other, eve)
	pa := s.asBearer(t, t1.AccessToken, principal.RoleMember)
	require.NoError(t, s.svc.RevokeCLISession(ctx, pa, s.asBearer(t, revoked.AccessToken, "").CLISessionID, "revoked"))
	_, err := s.pool.Exec(ctx, "UPDATE cli_session SET expires_at = now() - interval '1 second' WHERE id = $1",
		s.asBearer(t, expired.AccessToken, "").CLISessionID)
	require.NoError(t, err)
	s2 := s.asBearer(t, t2.AccessToken, "").CLISessionID

	list, err := s.svc.ListCLISessions(ctx, pa, nil)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{s2, pa.CLISessionID}, sessionIDs(list), "live only, newest first")
	assert.False(t, list[0].Current)
	assert.True(t, list[1].Current, "the caller's own Bearer session")
	assert.Equal(t, alice, list[1].UserID)
	assert.Equal(t, oauth.ClientCLI, list[1].ClientID)
	assert.Equal(t, "laptop", list[1].Hostname)
	assert.Equal(t, "127.0.0.1", list[1].IP.String())
	assert.False(t, list[1].ExpiresAt.IsZero())
	assert.False(t, list[1].AbsoluteExpiresAt.IsZero())

	own, err := s.svc.ListCLISessions(ctx, pa, &alice)
	require.NoError(t, err)
	assert.Len(t, own, 2, "own user_id needs no role")
	_, err = s.svc.ListCLISessions(ctx, pa, &bob)
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)

	actx, _ := loginCtx(t, s.svc, "adm")
	adm, _ := principal.From(actx)
	list, err = s.svc.ListCLISessions(ctx, adm, &alice)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{s2, pa.CLISessionID}, sessionIDs(list))
	assert.False(t, list[1].Current, "a cookie principal has no current CLI session")
	list, err = s.svc.ListCLISessions(ctx, adm, &eve)
	require.NoError(t, err)
	assert.Empty(t, list, "a user of another organization")

	res, err := s.cliSessions().ListCLISessions(ctx, bearer(&werbotv1.ListCLISessionsRequest{}, t1.AccessToken))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetSessions(), 2)
	got := res.Msg.GetSessions()[1]
	assert.Equal(t, pa.CLISessionID.String(), got.GetId())
	assert.True(t, got.GetCurrent())
	assert.Equal(t, "laptop", got.GetHostname())
	assert.Equal(t, "127.0.0.1", got.GetIp())
	bobID := bob.String()
	_, err = s.cliSessions().ListCLISessions(ctx, bearer(&werbotv1.ListCLISessionsRequest{UserId: &bobID}, t1.AccessToken))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = s.cliSessions().ListCLISessions(ctx, connect.NewRequest(&werbotv1.ListCLISessionsRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRevokeCLISession(t *testing.T) {
	s := newCLIStand(t)
	ctx := context.Background()
	org, other := seedOrg(t, s.pool, "Acme"), seedOrg(t, s.pool, "Other")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleMember)
	bob := seedUser(t, s.pool, org, "bob", principal.RoleMember)
	seedUser(t, s.pool, org, "adm", principal.RoleAdmin)
	eve := seedUser(t, s.pool, other, "eve", principal.RoleMember)
	t1, t2 := s.cliLogin(t, org, alice), s.cliLogin(t, org, alice)
	pa := s.asBearer(t, t1.AccessToken, principal.RoleMember)
	s2 := s.asBearer(t, t2.AccessToken, "").CLISessionID
	sb := s.asBearer(t, s.cliLogin(t, org, bob).AccessToken, "").CLISessionID
	se := s.asBearer(t, s.cliLogin(t, other, eve).AccessToken, "").CLISessionID

	require.NoError(t, s.svc.RevokeCLISession(ctx, pa, s2, "revoked"))
	var (
		reason, target, data string
		actor                uuid.UUID
	)
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT revoke_reason FROM cli_session WHERE id = $1`, s2).Scan(&reason))
	assert.Equal(t, "revoked", reason)
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT actor_user_id, target_type || ':' || target_id, data::text
		FROM audit_event WHERE action = 'cli_session.revoke'`).Scan(&actor, &target, &data))
	assert.Equal(t, alice, actor)
	assert.Equal(t, "cli_session:"+s2.String(), target)
	assert.JSONEq(t, `{"reason":"revoked"}`, data)
	_, ok := whoami(t, s.srv, bearerHeader(t2.AccessToken))
	assert.False(t, ok)

	assert.ErrorIs(t, s.svc.RevokeCLISession(ctx, pa, sb, "revoked"), apperr.ErrNotFound, "a member revokes only own sessions")
	actx, _ := loginCtx(t, s.svc, "adm")
	adm, _ := principal.From(actx)
	require.NoError(t, s.svc.RevokeCLISession(ctx, adm, sb, "revoked"), "an admin revokes any session in the organization")
	assert.ErrorIs(t, s.svc.RevokeCLISession(ctx, adm, sb, "revoked"), apperr.ErrNotFound, "already revoked")
	assert.ErrorIs(t, s.svc.RevokeCLISession(ctx, adm, se, "revoked"), apperr.ErrNotFound, "another organization")

	_, err := s.cliSessions().RevokeCLISession(ctx, bearer(&werbotv1.RevokeCLISessionRequest{Id: "x"}, t1.AccessToken))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = s.cliSessions().RevokeCLISession(ctx, bearer(&werbotv1.RevokeCLISessionRequest{Id: pa.CLISessionID.String()}, t1.AccessToken))
	require.NoError(t, err)
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT revoke_reason FROM cli_session WHERE id = $1`, pa.CLISessionID).Scan(&reason))
	assert.Equal(t, "revoked", reason, "the RPC passes reason revoked")
}
