package oidclogin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/identity/oidclogin/oidctest"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

// stand is werbot's OIDC routes in front of an oidctest.OP.
type stand struct {
	t     *testing.T
	pool  *pgxpool.Pool
	box   *secretbox.Box
	op    *oidctest.OP
	srv   *httptest.Server
	store *Store
	svc   *identity.Service
	cfg   Config
	org   uuid.UUID
	admin principal.Principal
	idp   IdP
}

func newStand(t *testing.T) *stand {
	t.Helper()
	pool, _ := dbtest.New(t)
	box, err := secretbox.New(bytes.Repeat([]byte{7}, 32), nil)
	require.NoError(t, err)
	s := &stand{t: t, pool: pool, box: box, op: oidctest.NewOP(t), store: NewStore(pool, box), org: dbtest.Org(t, pool)}
	s.admin = principal.Principal{OrgID: s.org, UserID: dbtest.User(t, pool, s.org, principal.RoleAdmin), Role: principal.RoleAdmin}
	var h http.Handler
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }))
	t.Cleanup(s.srv.Close)
	s.svc = identity.New(pool, &dbtest.Mails{}, identity.Config{WebURL: s.srv.URL, Box: box})
	s.cfg = Config{WebURL: s.srv.URL, HashKey: bytes.Repeat([]byte{1}, 32), BlockKey: bytes.Repeat([]byte{2}, 32), HTTPClient: s.op.Client()}
	h = Handler(s.svc, s.store, s.cfg)
	s.idp, err = s.store.Create(context.Background(), s.admin, IdP{Name: "corp", Issuer: s.op.URL, ClientID: oidctest.ClientID,
		ClientSecret: oidctest.ClientSecret, Scopes: DefaultScopes, Enabled: true})
	require.NoError(t, err)
	return s
}

// bob creates the member bob@example.com of the stand's organization.
func (s *stand) bob() uuid.UUID {
	s.t.Helper()
	id := dbtest.User(s.t, s.pool, s.org, principal.RoleMember)
	_, err := s.pool.Exec(context.Background(), "UPDATE users SET email = 'bob@example.com' WHERE id = $1", id)
	require.NoError(s.t, err)
	return id
}

// browser returns a client that keeps cookies and stops at every redirect.
func (s *stand) browser() *http.Client {
	c := *s.op.Client()
	c.Jar, _ = cookiejar.New(nil)
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

// reply is what a test reads of a response.
type reply struct {
	StatusCode int
	Location   string
	Cookies    []*http.Cookie
}

// get fetches u with c.
func (s *stand) get(c *http.Client, u string) reply {
	s.t.Helper()
	res, err := c.Get(u)
	require.NoError(s.t, err)
	require.NoError(s.t, res.Body.Close())
	return reply{StatusCode: res.StatusCode, Location: res.Header.Get("Location"), Cookies: res.Cookies()}
}

// login runs start → OP /authorize → callback for next; beforeCallback may
// change the callback URL. It returns the callback's reply.
func (s *stand) login(next string, beforeCallback func(*url.URL)) reply {
	s.t.Helper()
	c := s.browser()
	res := s.get(c, s.srv.URL+"/api/oidc/"+s.idp.ID.String()+"/start?next="+url.QueryEscape(next))
	require.Equal(s.t, http.StatusFound, res.StatusCode)
	res = s.get(c, res.Location)
	require.Equal(s.t, http.StatusFound, res.StatusCode, "OP /authorize")
	cb, err := url.Parse(res.Location)
	require.NoError(s.t, err)
	if beforeCallback != nil {
		beforeCallback(cb)
	}
	return s.get(c, cb.String())
}

// session is the principal of the session cookie res sets; ok is false without one.
func (s *stand) session(res reply) (p principal.Principal, ok bool) {
	s.t.Helper()
	for _, c := range res.Cookies {
		if c.Name == "werbot_session" && c.Value != "" {
			p, err := s.svc.AuthenticateSession(context.Background(), c.Value)
			require.NoError(s.t, err)
			return p, true
		}
	}
	return p, false
}
