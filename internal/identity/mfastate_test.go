package identity

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func setRequireMFA(t *testing.T, svc *Service, mode string) {
	t.Helper()
	_, err := svc.pool.Exec(context.Background(), "UPDATE org_setting SET require_mfa = $1", mode)
	require.NoError(t, err)
}

func TestEnrollmentRequired(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleAdmin)
	bob := seedUser(t, pool, org, "bob", principal.RoleMember)
	client := newAuthClient(t, svc)
	_, alice := loginCtx(t, svc, "alice")
	_, bobToken := loginCtx(t, svc, "bob")
	required := func(token string) bool {
		t.Helper()
		res, err := client.GetSession(context.Background(), withCookie(&werbotv1.GetSessionRequest{}, "__Host-werbot_session", token))
		require.NoError(t, err)
		return res.Msg.GetMfaEnrollmentRequired()
	}

	assert.False(t, required(alice), "off")
	setRequireMFA(t, svc, "admins")
	assert.True(t, required(alice), "admins: an admin without a factor")
	assert.False(t, required(bobToken), "admins: a member")
	setRequireMFA(t, svc, "all")
	assert.True(t, required(bobToken), "all: a member without a factor")
	dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, bob)
	assert.False(t, required(bobToken), "all: a member with a factor")
}

func TestAPIPathBlockedUntilEnrolled(t *testing.T) {
	svc, pool, _ := newTestService(t)
	alice := seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleMember)
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(NewHandler(svc)))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	srv := httptest.NewServer(Middleware(svc, nil)(mux))
	t.Cleanup(srv.Close)
	c := login(t, werbotv1connect.NewAuthServiceClient(srv.Client(), srv.URL))
	get := func(path string) (int, string) {
		t.Helper()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+path, http.NoBody)
		require.NoError(t, err)
		req.AddCookie(c)
		res, err := srv.Client().Do(req)
		require.NoError(t, err)
		defer func() { _ = res.Body.Close() }()
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		return res.StatusCode, string(body)
	}

	code, _ := get("/api/terminal")
	assert.Equal(t, http.StatusOK, code, "off")
	setRequireMFA(t, svc, "all")
	code, body := get("/api/terminal")
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "mfa enrollment required\n", body)
	for _, path := range []string{"/api/oidc/callback", "/profile/mfa"} {
		code, _ = get(path)
		assert.Equal(t, http.StatusOK, code, path)
	}
	dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, alice)
	code, _ = get("/api/terminal")
	assert.Equal(t, http.StatusOK, code, "enrolled")
}

func TestRequireMFASetting(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleAdmin)
	ctx, _ := loginCtx(t, svc, "alice")
	p, _ := principal.From(ctx)

	cur, err := svc.Settings(ctx, org)
	require.NoError(t, err)
	assert.Equal(t, "off", cur.RequireMFA)
	got, err := svc.UpdateSettings(ctx, p, OrgSettings{RequireMFA: "all"}, []string{"require_mfa"})
	require.NoError(t, err)
	assert.Equal(t, "all", got.RequireMFA)
	cur, err = svc.Settings(ctx, org)
	require.NoError(t, err)
	assert.Equal(t, "all", cur.RequireMFA)
	assert.Equal(t, 1, countAudit(t, pool, "settings.update", `{"fields":["require_mfa"]}`))

	_, err = svc.UpdateSettings(ctx, p, OrgSettings{RequireMFA: "some"}, []string{"require_mfa"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
}

func TestBearerMFAState(t *testing.T) {
	s := newCLIStand(t)
	org := seedOrg(t, s.pool, "Acme")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleMember)
	tr := s.cliLogin(t, org, alice)
	ctx := context.Background()

	p, st, err := s.svc.authenticateBearer(ctx, s.prov, "Bearer "+tr.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, mfaState{RequireMFA: "off"}, st)

	setRequireMFA(t, s.svc, "all")
	dbtest.Fixture{}.TOTP(t, s.pool, s.svc.cfg.Box, alice)
	verified := func(binding string, ago time.Duration) {
		_, err := s.pool.Exec(ctx, `INSERT INTO mfa_verification (org_id, user_id, binding, method, verified_at)
			VALUES ($1, $2, $3, 'totp', now() - make_interval(secs => $4::float8))`, org, alice, binding, ago.Seconds())
		require.NoError(t, err)
	}
	verified("cli:"+p.CLISessionID.String(), 10*time.Minute)
	verified("cli:"+p.CLISessionID.String(), time.Minute)
	verified("cli:"+uuid.NewString(), 0) // another CLI session's check
	verified("web:"+uuid.NewString(), 0)

	p, st, err = s.svc.authenticateBearer(ctx, s.prov, "Bearer "+tr.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "all", st.RequireMFA)
	assert.True(t, st.HasFactor)
	require.NotNil(t, st.FreshAt)
	assert.WithinDuration(t, time.Now().Add(-time.Minute), *st.FreshAt, 10*time.Second, "the newest check of this CLI session")
	assert.Nil(t, p.MFAAt, "MFAAt is a cookie session's only")
}
