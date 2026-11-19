package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

const inviteURLPrefix = "https://werbot.test/invite/"

// invite has ctx's user invite email and returns the raw token from the email.
func invite(ctx context.Context, t *testing.T, svc *Service, q *fakeQueue, email, role string) string {
	t.Helper()
	require.NoError(t, svc.InviteMember(ctx, email, role))
	mail := q.sent[len(q.sent)-1]
	require.Equal(t, "invite", mail.Template)
	require.True(t, strings.HasPrefix(mail.Data["url"], inviteURLPrefix), mail.Data["url"])
	return strings.TrimPrefix(mail.Data["url"], inviteURLPrefix)
}

func memberRole(t *testing.T, pool *pgxpool.Pool, orgID, userID uuid.UUID) string {
	t.Helper()
	var role string
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT role FROM membership WHERE org_id = $1 AND user_id = $2", orgID, userID).Scan(&role))
	return role
}

func TestInviteAndAcceptNewUser(t *testing.T) {
	svc, pool, q := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")

	raw := invite(ctx, t, svc, q, "Bob@Example.com", principal.RoleMember)
	mail := q.sent[0]
	assert.Equal(t, "bob@example.com", mail.To)
	assert.Equal(t, "Acme", mail.Data["org"])
	assert.Equal(t, "alice", mail.Data["inviter"])

	members, invites, err := svc.ListMembers(ctx)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Len(t, invites, 1)
	assert.Equal(t, "bob@example.com", invites[0].Email)
	assert.Equal(t, principal.RoleMember, invites[0].Role)

	bg := context.Background()
	in := AcceptInput{Token: raw, Login: "bob", Name: "Bob", Password: testPassword}
	token, err := svc.AcceptInvite(bg, in, "test")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	p, err := svc.AuthenticateSession(bg, token)
	require.NoError(t, err)
	assert.Equal(t, org, p.OrgID)
	assert.Equal(t, principal.RoleMember, p.Role)

	_, err = svc.AcceptInvite(bg, in, "test")
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	members, invites, err = svc.ListMembers(ctx)
	require.NoError(t, err)
	assert.Len(t, members, 2)
	assert.Empty(t, invites)
	assert.Subset(t, auditActions(t, pool), []string{"member.invite", "member.accept"})
}

func TestAcceptInviteNewUserNeedsFields(t *testing.T) {
	svc, pool, q := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")
	raw := invite(ctx, t, svc, q, "bob@example.com", principal.RoleMember)

	bg := context.Background()
	_, err := svc.AcceptInvite(bg, AcceptInput{Token: raw, Password: testPassword}, "test")
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
	for _, login := range []string{"Bob", "b", "-bob"} {
		_, err = svc.AcceptInvite(bg, AcceptInput{Token: raw, Login: login, Name: "Bob", Password: testPassword}, "test")
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, login)
	}
	_, err = svc.AcceptInvite(bg, AcceptInput{Token: raw, Login: "bob", Name: "Bob", Password: "short"}, "test")
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
	_, err = svc.AcceptInvite(bg, AcceptInput{Token: raw, Login: "alice", Name: "Bob", Password: testPassword}, "test")
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists, "login taken")
	_, err = svc.AcceptInvite(bg, AcceptInput{Token: raw, Login: "bob", Name: "Bob", Password: testPassword}, "test")
	require.NoError(t, err, "failed attempts leave the token unspent")
}

func TestMemberCannotInvite(t *testing.T) {
	svc, pool, q := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	seedUser(t, pool, org, "carol", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "carol")

	assert.ErrorIs(t, svc.InviteMember(ctx, "bob@example.com", principal.RoleMember), apperr.ErrPermissionDenied)
	assert.Empty(t, q.sent)
	_, _, err := svc.ListMembers(ctx)
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
}

func TestAdminCannotGrantOwner(t *testing.T) {
	svc, pool, q := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	alice := seedUser(t, pool, org, "alice", principal.RoleOwner)
	seedUser(t, pool, org, "ann", principal.RoleAdmin)
	bob := seedUser(t, pool, org, "bob", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "ann")

	assert.ErrorIs(t, svc.InviteMember(ctx, "x@example.com", principal.RoleOwner), apperr.ErrPermissionDenied)
	assert.ErrorIs(t, svc.UpdateMemberRole(ctx, bob, principal.RoleOwner), apperr.ErrPermissionDenied)
	assert.ErrorIs(t, svc.UpdateMemberRole(ctx, alice, principal.RoleAdmin), apperr.ErrPermissionDenied)
	assert.ErrorIs(t, svc.RemoveMember(ctx, alice), apperr.ErrPermissionDenied)
	assert.Empty(t, q.sent)
	assert.Equal(t, principal.RoleOwner, memberRole(t, pool, org, alice))

	assert.ErrorIs(t, svc.UpdateMemberRole(ctx, bob, "root"), apperr.ErrInvalidArgument)
	require.NoError(t, svc.UpdateMemberRole(ctx, bob, principal.RoleAdmin))
	assert.Equal(t, principal.RoleAdmin, memberRole(t, pool, org, bob))
}

func TestInviteExistingMember(t *testing.T) {
	svc, pool, q := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	seedUser(t, pool, org, "bob", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "alice")

	assert.ErrorIs(t, svc.InviteMember(ctx, " BOB@Example.COM ", principal.RoleMember), apperr.ErrAlreadyExists)
	assert.Empty(t, q.sent)
}

func TestAcceptInviteExistingUser(t *testing.T) {
	svc, pool, q := newTestService(t)
	org1, org2 := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Other")
	seedUser(t, pool, org1, "alice", principal.RoleOwner)
	dave := seedUser(t, pool, org2, "dave", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")
	raw := invite(ctx, t, svc, q, "Dave@example.com", principal.RoleAdmin)

	bg := context.Background()
	_, err := svc.AcceptInvite(bg, AcceptInput{Token: raw, Password: "wrong password!"}, "test")
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
	token, err := svc.AcceptInvite(bg, AcceptInput{Token: raw, Password: testPassword}, "test")
	require.NoError(t, err, "a wrong password leaves the token unspent")
	p, err := svc.AuthenticateSession(bg, token)
	require.NoError(t, err)
	assert.Equal(t, principal.Principal{UserID: dave, OrgID: org1, Role: principal.RoleAdmin, SessionID: p.SessionID, AuthMethod: principal.AuthMethodCookie}, p)
	assert.Equal(t, principal.RoleOwner, memberRole(t, pool, org2, dave))
}

func TestAcceptInviteWrongSessionUser(t *testing.T) {
	svc, pool, q := newTestService(t)
	org1, org2 := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Other")
	seedUser(t, pool, org1, "alice", principal.RoleOwner)
	seedUser(t, pool, org1, "carol", principal.RoleMember)
	dave := seedUser(t, pool, org2, "dave", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "alice")
	raw := invite(ctx, t, svc, q, "dave@example.com", principal.RoleMember)

	carol, _ := loginCtx(t, svc, "carol")
	_, err := svc.AcceptInvite(carol, AcceptInput{Token: raw}, "test")
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = svc.AcceptInvite(context.Background(), AcceptInput{Token: raw, Password: testPassword}, "test")
	require.NoError(t, err)
	assert.Equal(t, principal.RoleMember, memberRole(t, pool, org1, dave))
}

func TestAcceptInviteUnderSession(t *testing.T) {
	svc, pool, q := newTestService(t)
	org1, org2 := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Other")
	seedUser(t, pool, org1, "alice", principal.RoleOwner)
	dave := seedUser(t, pool, org2, "dave", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "alice")
	first := invite(ctx, t, svc, q, "dave@example.com", principal.RoleAdmin)
	second := invite(ctx, t, svc, q, "dave@example.com", principal.RoleMember)

	daveCtx, _ := loginCtx(t, svc, "dave")
	token, err := svc.AcceptInvite(daveCtx, AcceptInput{Token: first}, "test")
	require.NoError(t, err)
	assert.Empty(t, token, "a signed-in user keeps the current session")
	assert.Equal(t, principal.RoleAdmin, memberRole(t, pool, org1, dave))
	_, err = svc.AcceptInvite(daveCtx, AcceptInput{Token: second}, "test")
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)
}

func TestLastOwnerCannotBeDemotedOrRemoved(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	alice := seedUser(t, pool, org, "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")

	assert.ErrorIs(t, svc.UpdateMemberRole(ctx, alice, principal.RoleAdmin), apperr.ErrFailedPrecondition)
	assert.ErrorIs(t, svc.RemoveMember(ctx, alice), apperr.ErrFailedPrecondition)
	assert.Equal(t, principal.RoleOwner, memberRole(t, pool, org, alice))
}

func TestConcurrentOwnerDemotionKeepsOneOwner(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	a := seedUser(t, pool, org, "alice", principal.RoleOwner)
	b := seedUser(t, pool, org, "bob", principal.RoleOwner)
	ctxA, _ := loginCtx(t, svc, "alice")
	ctxB, _ := loginCtx(t, svc, "bob")

	for range 5 {
		_, err := pool.Exec(context.Background(), "UPDATE membership SET role = 'owner'")
		require.NoError(t, err)
		var errs [2]error
		var wg sync.WaitGroup
		wg.Go(func() { errs[0] = svc.UpdateMemberRole(ctxA, b, principal.RoleAdmin) })
		wg.Go(func() { errs[1] = svc.UpdateMemberRole(ctxB, a, principal.RoleAdmin) })
		wg.Wait()

		require.True(t, (errs[0] == nil) != (errs[1] == nil), "exactly one fails: %v", errs)
		for _, err := range errs {
			if err != nil {
				// The loser's session still says owner; under the owner lock it is not.
				assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
			}
		}
		var owners int
		require.NoError(t, pool.QueryRow(context.Background(),
			"SELECT count(*) FROM membership WHERE role = 'owner'").Scan(&owners))
		require.Equal(t, 1, owners)
	}
}

func TestRemovedMemberSessionStopsWorking(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	bob := seedUser(t, pool, org, "bob", principal.RoleMember)
	carol := seedUser(t, pool, org, "carol", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "alice")
	_, bobToken := loginCtx(t, svc, "bob")
	_, carolToken := loginCtx(t, svc, "carol")

	bg := context.Background()
	require.NoError(t, svc.RemoveMember(ctx, bob))
	_, err := svc.AuthenticateSession(bg, bobToken)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)

	require.NoError(t, svc.UpdateMemberRole(ctx, carol, principal.RoleAdmin))
	p, err := svc.AuthenticateSession(bg, carolToken)
	require.NoError(t, err)
	assert.Equal(t, principal.RoleAdmin, p.Role)

	var from, to string
	require.NoError(t, pool.QueryRow(bg,
		"SELECT data->>'from', data->>'to' FROM audit_event WHERE action = 'member.role_update'").Scan(&from, &to))
	assert.Equal(t, []string{principal.RoleMember, principal.RoleAdmin}, []string{from, to})
	assert.Contains(t, auditActions(t, pool), "member.remove")
}

func TestCrossOrgIsolationMembers(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org1, org2 := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Other")
	seedUser(t, pool, org1, "alice", principal.RoleOwner)
	eve := seedUser(t, pool, org2, "eve", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "alice")

	assert.ErrorIs(t, svc.UpdateMemberRole(ctx, eve, principal.RoleAdmin), apperr.ErrNotFound)
	assert.ErrorIs(t, svc.RemoveMember(ctx, eve), apperr.ErrNotFound)
	members, _, err := svc.ListMembers(ctx)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "alice", members[0].User.Login)
	assert.Equal(t, principal.RoleMember, memberRole(t, pool, org2, eve))
}

func TestMemberRPC(t *testing.T) {
	svc, pool, q := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(NewHandler(svc)))
	mux.Handle(werbotv1connect.NewMemberServiceHandler(NewHandler(svc)))
	srv := httptest.NewServer(Middleware(svc)(mux))
	t.Cleanup(srv.Close)
	members := werbotv1connect.NewMemberServiceClient(srv.Client(), srv.URL)
	c := login(t, werbotv1connect.NewAuthServiceClient(srv.Client(), srv.URL))
	bg := context.Background()

	_, err := members.ListMembers(bg, connect.NewRequest(&werbotv1.ListMembersRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = members.InviteMember(bg, withCookie(&werbotv1.InviteMemberRequest{Email: "bob@example.com", Role: werbotv1.Role_ROLE_ADMIN}, c.Name, c.Value))
	require.NoError(t, err)
	res, err := members.ListMembers(bg, withCookie(&werbotv1.ListMembersRequest{}, c.Name, c.Value))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetInvites(), 1)
	assert.Equal(t, werbotv1.Role_ROLE_ADMIN, res.Msg.GetInvites()[0].GetRole())
	assert.Equal(t, werbotv1.Role_ROLE_OWNER, res.Msg.GetMembers()[0].GetRole())

	raw := strings.TrimPrefix(q.sent[0].Data["url"], inviteURLPrefix)
	acc, err := members.AcceptInvite(bg, connect.NewRequest(&werbotv1.AcceptInviteRequest{Token: raw, Login: "bob", Name: "Bob", Password: testPassword}))
	require.NoError(t, err)
	cookie, err := http.ParseSetCookie(acc.Header().Get("Set-Cookie"))
	require.NoError(t, err)
	assert.Equal(t, c.Name, cookie.Name)
	assert.Len(t, cookie.Value, 43)
}
