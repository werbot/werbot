package oauth_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/identity/oidclogin"
	"github.com/werbot/werbot/internal/identity/oidclogin/oidctest"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// idpStand is a provider whose refreshes ask the member's IdP (an oidctest
// OP) through oidclogin.RecheckUser, as the server wires it.
func idpStand(t *testing.T) (stand, *oidctest.OP) {
	t.Helper()
	ctx := context.Background()
	pool, _ := dbtest.New(t)
	fx := dbtest.Seed(t, pool)
	box := testBox(t)
	key, err := oauth.LoadSigningKey(ctx, pool, box)
	require.NoError(t, err)
	op := oidctest.NewOP(t)
	store := oidclogin.NewStore(pool, box)
	idp, err := store.Create(ctx, fx.Principal(fx.OwnerID), oidclogin.IdP{Name: "corp", Issuer: op.URL,
		ClientID: oidctest.ClientID, ClientSecret: oidctest.ClientSecret, Scopes: oidclogin.DefaultScopes, Enabled: true})
	require.NoError(t, err)
	sealed, keyID, err := store.SealRefresh(op.Refresh("sub-m"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO user_idp_link (user_id, idp_id, org_id, subject, refresh_ciphertext, key_id)
		VALUES ($1, $2, $3, 'sub-m', $4, $5)`, fx.MemberID, idp.ID, fx.OrgID, sealed, keyID)
	require.NoError(t, err)

	svc := identity.New(pool, &dbtest.Mails{}, identity.Config{WebURL: testWebURL, Box: box})
	cfg := oidclogin.Config{WebURL: testWebURL, HTTPClient: op.Client()}
	check := func(ctx context.Context, orgID, userID uuid.UUID) error {
		return oidclogin.RecheckUser(ctx, store, svc, cfg, orgID, userID)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p, err := oauth.New(pool, key, dbUsers{pool}, nil, oauth.Config{Issuer: "http://" + ln.Addr().String(), WebURL: testWebURL, IdPCheck: check})
	require.NoError(t, err)
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: clientip.Middleware(nil)(p.Handler())}} //nolint:gosec // test server
	srv.Start()
	t.Cleanup(srv.Close)
	return stand{p: p, key: key, pool: pool, srv: srv, fx: fx}, op
}

func TestCLIRefreshRechecksIdP(t *testing.T) {
	s, op := idpStand(t)
	tr := newSession(t, s)
	op.Disable("sub-m")

	tr = refresh(t, s, tr.RefreshToken)
	require.Equal(t, http.StatusOK, tr.Status, "a link checked just now is not asked")

	exec(t, s, "UPDATE user_idp_link SET last_checked_at = now() - interval '2 hours'")
	op.SetDown(true)
	tr = refresh(t, s, tr.RefreshToken)
	require.Equal(t, http.StatusOK, tr.Status, "an unreachable IdP does not stop the renewal")
	reason, _ := session(t, s)
	assert.Empty(t, reason)

	op.SetDown(false)
	exec(t, s, "UPDATE user_idp_link SET last_checked_at = now() - interval '2 hours'") // the last renewal claimed the link
	tr = refresh(t, s, tr.RefreshToken)
	assert.Equal(t, http.StatusBadRequest, tr.Status)
	assert.Equal(t, "invalid_grant", tr.Error)
	reason, _ = session(t, s)
	assert.Equal(t, "idp", reason)
}
