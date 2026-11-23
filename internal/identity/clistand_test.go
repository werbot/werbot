package identity

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// cliStand serves identity and the CLI authorization server behind
// Middleware(svc, prov), as serve does; /whoami answers the subject as JSON.
type cliStand struct {
	svc  *Service
	pool *pgxpool.Pool
	q    *dbtest.Mails
	prov *oauth.Provider
	key  *oauth.SigningKey
	srv  *httptest.Server
}

func newCLIStand(t *testing.T) cliStand {
	t.Helper()
	svc, pool, q := newTestService(t)
	key, err := oauth.LoadSigningKey(context.Background(), pool, svc.cfg.Box)
	require.NoError(t, err)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	prov, err := oauth.New(pool, key, svc, nil, oauth.Config{Issuer: "http://" + ln.Addr().String(), WebURL: svc.cfg.WebURL})
	require.NoError(t, err)
	h := NewHandler(svc)
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(h))
	mux.Handle(werbotv1connect.NewCLISessionServiceHandler(h))
	mux.Handle(werbotv1connect.NewCertServiceHandler(h))
	mux.Handle("/oauth/", prov.Handler())
	mux.HandleFunc("/whoami", whoamiHandler)
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: clientip.Middleware(nil)(Middleware(svc, prov)(mux))}} //nolint:gosec // test server
	srv.Start()
	t.Cleanup(srv.Close)
	return cliStand{svc: svc, pool: pool, q: q, prov: prov, key: key, srv: srv}
}

func whoamiHandler(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.From(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	_ = json.NewEncoder(w).Encode(p)
}

type cliTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
}

func (s cliStand) postForm(t *testing.T, path string, form url.Values, out any) int {
	t.Helper()
	res, err := s.srv.Client().PostForm(s.srv.URL+path, form)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.NoError(t, json.NewDecoder(res.Body).Decode(out))
	return res.StatusCode
}

// cliLogin signs userID of orgID in over the device flow: one cli_session.
func (s cliStand) cliLogin(t *testing.T, orgID, userID uuid.UUID) cliTokens {
	t.Helper()
	var dr struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	require.Equal(t, http.StatusOK, s.postForm(t, "/oauth/device_authorization", url.Values{
		"client_id": {oauth.ClientCLI}, "scope": {"openid offline_access"}, "werbot_hostname": {"laptop"},
	}, &dr))
	browser := principal.Principal{UserID: userID, OrgID: orgID, AuthMethod: principal.AuthMethodCookie}
	require.NoError(t, s.prov.ApproveDeviceCode(context.Background(), browser, dr.UserCode))
	var tr cliTokens
	require.Equal(t, http.StatusOK, s.postForm(t, "/oauth/token", url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "client_id": {oauth.ClientCLI}, "device_code": {dr.DeviceCode},
	}, &tr), tr.Error)
	return tr
}

// asBearer is the principal the middleware builds for the access token.
func (s cliStand) asBearer(t *testing.T, token, role string) principal.Principal {
	t.Helper()
	c, err := s.prov.VerifyAccessToken(context.Background(), token)
	require.NoError(t, err)
	return principal.Principal{UserID: c.UserID, OrgID: c.OrgID, Role: role, AuthMethod: principal.AuthMethodOAuth, CLISessionID: c.CLISessionID}
}

func (s cliStand) refresh(t *testing.T, token string) cliTokens {
	t.Helper()
	var tr cliTokens
	s.postForm(t, "/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {oauth.ClientCLI}, "refresh_token": {token}}, &tr)
	return tr
}

func (s cliStand) auth() werbotv1connect.AuthServiceClient {
	return werbotv1connect.NewAuthServiceClient(s.srv.Client(), s.srv.URL)
}

func (s cliStand) cliSessions() werbotv1connect.CLISessionServiceClient {
	return werbotv1connect.NewCLISessionServiceClient(s.srv.Client(), s.srv.URL)
}

func (s cliStand) certs() werbotv1connect.CertServiceClient {
	return werbotv1connect.NewCertServiceClient(s.srv.Client(), s.srv.URL)
}

// whoami returns the subject the middleware gives a request with header.
func whoami(t *testing.T, srv *httptest.Server, header http.Header) (principal.Principal, bool) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/whoami", nil)
	require.NoError(t, err)
	req.Header = header
	res, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	var p principal.Principal
	if res.StatusCode != http.StatusOK {
		return p, false
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&p))
	return p, true
}

func bearerHeader(token string) http.Header {
	return http.Header{"Authorization": {"Bearer " + token}}
}

func bearer[T any](msg *T, token string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+token)
	return req
}
