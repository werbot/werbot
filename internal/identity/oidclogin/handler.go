package oidclogin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"

	"github.com/werbot/werbot/internal/identity"
)

// Login cookies besides rp's "state" and "pkce"; all live 10 minutes on /api/oidc/.
const (
	nonceCookie = "werbot_oidc_nonce"
	nextCookie  = "werbot_oidc_next"
	cookieTTL   = 10 * 60
	loginFailed = "/login?error=oidc"
)

type handler struct {
	svc     *identity.Service
	store   IdPStore
	cfg     Config
	cookies *httphelper.CookieHandler
}

// Handler serves GET /api/oidc/{idp_id}/start?next= and GET /api/oidc/callback.
// Start sends the browser to the IdP with state "<idp_id>.<random>", PKCE
// S256 and a nonce, kept in signed and encrypted cookies; the callback checks
// all three, links the account by verified email only and sets the session
// cookie (no Werbot second factor: owner decision O1).
func Handler(s *identity.Service, store IdPStore, cfg Config) http.Handler {
	opts := []httphelper.CookieHandlerOpt{httphelper.WithMaxAge(cookieTTL), httphelper.WithPath("/api/oidc/")}
	if !cfg.SecureCookies {
		opts = append(opts, httphelper.WithUnsecure())
	}
	h := &handler{svc: s, store: store, cfg: cfg, cookies: httphelper.NewCookieHandler(cfg.HashKey, cfg.BlockKey, opts...)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/oidc/{idp_id}/start", h.start)
	mux.HandleFunc("GET /api/oidc/callback", h.callback)
	return mux
}

// start redirects to the IdP; an unknown or disabled IdP is 404, an
// unreachable one goes back to the login page.
func (h *handler) start(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("idp_id"))
	var idp IdP
	if err == nil {
		idp, err = h.store.Get(r.Context(), id)
	}
	if err != nil || !idp.Enabled {
		http.NotFound(w, r)
		return
	}
	party, err := h.party(r.Context(), idp)
	if err != nil {
		h.fail(w, r, idp, "discovery", err)
		return
	}
	nonce := random()
	if err := errors.Join(h.cookies.SetCookie(w, nonceCookie, nonce),
		h.cookies.SetCookie(w, nextCookie, safeNext(r.URL.Query().Get("next")))); err != nil {
		h.fail(w, r, idp, "cookie", err)
		return
	}
	state := func() string { return idp.ID.String() + "." + random() }
	rp.AuthURLHandler(state, party, rp.WithURLParam("nonce", nonce))(w, r)
}

// callback checks the state cookie before anything else: a foreign or
// missing state, a missing nonce cookie, a PKCE or nonce mismatch are 400.
func (h *handler) callback(w http.ResponseWriter, r *http.Request) {
	state, err := h.cookies.CheckQueryCookie(r, "state")
	if err != nil {
		badRequest(w)
		return
	}
	nonce, err := h.cookies.CheckCookie(r, nonceCookie)
	if err != nil {
		badRequest(w)
		return
	}
	prefix, _, _ := strings.Cut(state, ".")
	id, err := uuid.Parse(prefix)
	var idp IdP
	if err == nil {
		idp, err = h.store.Get(r.Context(), id)
	}
	if err != nil || !idp.Enabled {
		http.Redirect(w, r, loginFailed, http.StatusFound)
		return
	}
	party, err := h.party(r.Context(), idp,
		rp.WithVerifierOpts(rp.WithNonce(func(context.Context) string { return nonce })),
		rp.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, _, _, _ string) {
			http.Redirect(w, r, loginFailed, http.StatusFound)
		}),
		rp.WithUnauthorizedHandler(func(w http.ResponseWriter, _ *http.Request, _, _ string) { badRequest(w) }))
	if err != nil {
		h.fail(w, r, idp, "discovery", err)
		return
	}
	rp.CodeExchangeHandler(h.finish(idp), party)(w, r)
}

// party runs discovery of idp (no cache: an unreachable IdP refuses the login).
func (h *handler) party(ctx context.Context, idp IdP, opts ...rp.Option) (rp.RelyingParty, error) {
	return relyingParty(ctx, h.cfg, idp, append(opts, rp.WithPKCE(h.cookies))...)
}

// relyingParty runs discovery of idp over the client of cfg.
func relyingParty(ctx context.Context, cfg Config, idp IdP, opts ...rp.Option) (rp.RelyingParty, error) {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	opts = append(opts, rp.WithHTTPClient(client))
	return rp.NewRelyingPartyOIDC(ctx, idp.Issuer, idp.ClientID, idp.ClientSecret, cfg.redirectURI(), idp.Scopes, opts...)
}

// fail logs why the login of idp stopped (never a token or secret) and
// sends the browser back to the login page.
func (h *handler) fail(w http.ResponseWriter, r *http.Request, idp IdP, step string, err error) {
	slog.Default().Warn("oidc login failed", "idp_id", idp.ID, "step", step, "err", err)
	http.Redirect(w, r, loginFailed, http.StatusFound)
}

func badRequest(w http.ResponseWriter) { http.Error(w, "oidc login refused", http.StatusBadRequest) }

// random is 32 random bytes, base64url.
func random() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// safeNext mirrors web/src/lib/next.ts: a path from the root of this origin
// without "//", "/\" or control characters; anything else is "/".
func safeNext(s string) string {
	if !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/\\") ||
		strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return "/"
	}
	return s
}
