package oauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// consentClient serves OAuthConsentService of s with p as the caller (nil: none).
func consentClient(t *testing.T, s stand, p *principal.Principal) werbotv1connect.OAuthConsentServiceClient {
	t.Helper()
	path, h := werbotv1connect.NewOAuthConsentServiceHandler(oauth.NewConsentHandler(s.p))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p != nil {
			r = r.WithContext(principal.With(r.Context(), *p))
		}
		h.ServeHTTP(w, r)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return werbotv1connect.NewOAuthConsentServiceClient(srv.Client(), srv.URL)
}

// TestConsentRequiresCookie: a CLI token (Bearer) never approves a sign-in;
// without a subject the RPCs are unauthenticated.
func TestConsentRequiresCookie(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	_, challenge := pkce()
	id := authorize(t, s.srv, challenge, nil)

	bearer := s.fx.Principal(s.fx.MemberID)
	bearer.AuthMethod = principal.AuthMethodOAuth
	_, err := s.p.GetAuthRequest(ctx, bearer, id)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = s.p.ApproveAuthRequest(ctx, bearer, id)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = s.p.DenyAuthRequest(ctx, bearer, id)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = s.p.GetAuthRequest(ctx, s.fx.Principal(s.fx.MemberID), id) // no AuthMethod: not a cookie
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	dr := startDevice(t, s.srv)
	_, err = s.p.LookupDeviceCode(ctx, bearer, dr.UserCode)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	require.ErrorIs(t, s.p.ApproveDeviceCode(ctx, bearer, dr.UserCode), apperr.ErrPermissionDenied)
	require.ErrorIs(t, s.p.DenyDeviceCode(ctx, bearer, dr.UserCode), apperr.ErrPermissionDenied)

	idReq := func() *connect.Request[werbotv1.GetAuthRequestRequest] {
		return connect.NewRequest(&werbotv1.GetAuthRequestRequest{Id: id.String()})
	}
	for name, c := range map[string]struct {
		p    *principal.Principal
		code connect.Code
	}{"anonymous": {nil, connect.CodeUnauthenticated}, "bearer": {&bearer, connect.CodePermissionDenied}} {
		cl := consentClient(t, s, c.p)
		_, err = cl.GetAuthRequest(ctx, idReq())
		assert.Equal(t, c.code, connect.CodeOf(err), name)
		_, err = cl.ApproveAuthRequest(ctx, connect.NewRequest(&werbotv1.ApproveAuthRequestRequest{Id: id.String()}))
		assert.Equal(t, c.code, connect.CodeOf(err), name)
		_, err = cl.DenyAuthRequest(ctx, connect.NewRequest(&werbotv1.DenyAuthRequestRequest{Id: id.String()}))
		assert.Equal(t, c.code, connect.CodeOf(err), name)
		_, err = cl.LookupDeviceCode(ctx, connect.NewRequest(&werbotv1.LookupDeviceCodeRequest{UserCode: dr.UserCode}))
		assert.Equal(t, c.code, connect.CodeOf(err), name)
		_, err = cl.ApproveDeviceCode(ctx, connect.NewRequest(&werbotv1.ApproveDeviceCodeRequest{UserCode: dr.UserCode}))
		assert.Equal(t, c.code, connect.CodeOf(err), name)
		_, err = cl.DenyDeviceCode(ctx, connect.NewRequest(&werbotv1.DenyDeviceCodeRequest{UserCode: dr.UserCode}))
		assert.Equal(t, c.code, connect.CodeOf(err), name)
	}
	assert.Equal(t, "pending", deviceStatus(t, s, dr.UserCode))

	cookie := cookieAs(s.fx, s.fx.MemberID)
	cl := consentClient(t, s, &cookie)
	got, err := cl.GetAuthRequest(ctx, idReq())
	require.NoError(t, err)
	assert.Equal(t, oauth.ClientCLI, got.Msg.GetClientId())
	assert.Equal(t, "127.0.0.1", got.Msg.GetClientIp())
	assert.Equal(t, []string{"openid", "offline_access"}, got.Msg.GetScopes())
	ok, err := cl.ApproveAuthRequest(ctx, connect.NewRequest(&werbotv1.ApproveAuthRequestRequest{Id: id.String()}))
	require.NoError(t, err)
	assert.Equal(t, testWebURL+"/oauth/authorize/callback?id="+id.String(), ok.Msg.GetRedirectUrl())
	_, err = cl.GetAuthRequest(ctx, idReq()) // approved: no longer pending
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	dev, err := cl.LookupDeviceCode(ctx, connect.NewRequest(&werbotv1.LookupDeviceCodeRequest{UserCode: dr.UserCode}))
	require.NoError(t, err)
	assert.Equal(t, "build-01", dev.Msg.GetClientHostname())
	assert.Equal(t, "127.0.0.1", dev.Msg.GetClientIp())
	_, err = cl.LookupDeviceCode(ctx, connect.NewRequest(&werbotv1.LookupDeviceCodeRequest{UserCode: "AAAA-AAAA"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = cl.DenyDeviceCode(ctx, connect.NewRequest(&werbotv1.DenyDeviceCodeRequest{UserCode: dr.UserCode}))
	require.NoError(t, err)
	_, err = cl.ApproveDeviceCode(ctx, connect.NewRequest(&werbotv1.ApproveDeviceCodeRequest{UserCode: dr.UserCode}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestAuthRequestExpires: an ended request can be neither approved nor exchanged.
func TestAuthRequestExpires(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	_, challenge := pkce()
	id := authorize(t, s.srv, challenge, nil)
	_, err := s.pool.Exec(ctx, "UPDATE oauth_auth_request SET expires_at = now() - interval '1 second' WHERE id = $1", id)
	require.NoError(t, err)
	_, err = s.p.ApproveAuthRequest(ctx, cookieAs(s.fx, s.fx.MemberID), id)
	require.ErrorIs(t, err, apperr.ErrNotFound)
	_, err = s.p.GetAuthRequest(ctx, cookieAs(s.fx, s.fx.MemberID), id)
	require.ErrorIs(t, err, apperr.ErrNotFound)

	code, verifier := login(t, s, s.fx.MemberID)
	_, err = s.pool.Exec(ctx, "UPDATE oauth_auth_request SET expires_at = now() - interval '1 second'")
	require.NoError(t, err)
	tr := exchangeCode(t, s.srv, code, verifier, testRedirect)
	assert.Equal(t, http.StatusBadRequest, tr.Status)
	assert.Equal(t, "invalid_grant", tr.Error)
}

// TestDenyRedirectsWithError: Deny hands the CLI interaction_required at once.
func TestDenyRedirectsWithError(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	_, challenge := pkce()
	id := authorize(t, s.srv, challenge, nil)
	redirect, err := s.p.DenyAuthRequest(ctx, cookieAs(s.fx, s.fx.MemberID), id)
	require.NoError(t, err)
	loc := callback(t, s.srv, redirect)
	assert.Equal(t, testRedirect, loc.Scheme+"://"+loc.Host+loc.Path)
	assert.Equal(t, "interaction_required", loc.Query().Get("error"))
	assert.Equal(t, "st-1", loc.Query().Get("state"))
	assert.Empty(t, loc.Query().Get("code"))
	_, err = s.p.ApproveAuthRequest(ctx, cookieAs(s.fx, s.fx.MemberID), id)
	require.ErrorIs(t, err, apperr.ErrNotFound)
	_, err = s.p.DenyAuthRequest(ctx, cookieAs(s.fx, s.fx.MemberID), uuid.New())
	require.ErrorIs(t, err, apperr.ErrNotFound)
}

// TestCallbackBeforeApproval: the CLI learns that consent is missing (R10).
func TestCallbackBeforeApproval(t *testing.T) {
	s := newStand(t)
	_, challenge := pkce()
	id := authorize(t, s.srv, challenge, nil)
	loc := callback(t, s.srv, testWebURL+"/oauth/authorize/callback?id="+id.String())
	assert.Equal(t, "interaction_required", loc.Query().Get("error"))
	assert.Empty(t, loc.Query().Get("code"))
}
