// Package oidctest is an OpenID provider for tests: discovery, JWKS,
// /authorize (signs in the user of SetUser at once), /token (code with PKCE
// S256, refresh) and /userinfo, served over TLS by httptest.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// The client the OP accepts.
const (
	ClientID     = "werbot"
	ClientSecret = "op-client-secret"
)

type grant struct {
	sub, nonce, challenge string
}

// OP is the test provider; its zero users sign in nobody.
type OP struct {
	*httptest.Server
	signer jose.Signer
	jwks   jose.JSONWebKeySet

	mu        sync.Mutex
	sub       string
	claims    map[string]map[string]any
	disabled  map[string]bool
	down      bool
	nonce     *string
	method    string
	codes     map[string]grant
	access    map[string]string // access token → subject
	refreshes map[string]string // refresh token → subject
}

// NewOP starts an OP closed with t.
func NewOP(t testing.TB) *OP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	o := &OP{signer: signer, claims: map[string]map[string]any{}, disabled: map[string]bool{},
		codes: map[string]grant{}, access: map[string]string{}, refreshes: map[string]string{},
		jwks: jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", o.discovery)
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, o.jwks) })
	mux.HandleFunc("GET /authorize", o.authorize)
	mux.HandleFunc("POST /token", o.token)
	mux.HandleFunc("GET /userinfo", o.userinfo)
	o.Server = httptest.NewTLSServer(o.unlessDown(mux))
	t.Cleanup(o.Close)
	return o
}

// SetUser makes sub with claims (email, email_verified, groups, …) the user
// /authorize signs in.
func (o *OP) SetUser(sub string, claims map[string]any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sub, o.claims[sub] = sub, claims
}

// Refresh is a new refresh token of sub, as if sub had signed in.
func (o *OP) Refresh(sub string) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	t := random()
	o.refreshes[t] = sub
	return t
}

// Disable makes refreshes of sub fail with invalid_grant.
func (o *OP) Disable(sub string) { o.mu.Lock(); o.disabled[sub] = true; o.mu.Unlock() }

// SetDown makes every endpoint answer 503.
func (o *OP) SetDown(down bool) { o.mu.Lock(); o.down = down; o.mu.Unlock() }

// SetNonce puts n into the ID tokens instead of the nonce of the request.
func (o *OP) SetNonce(n string) { o.mu.Lock(); o.nonce = &n; o.mu.Unlock() }

// ChallengeMethod is the code_challenge_method of the last /authorize.
func (o *OP) ChallengeMethod() string { o.mu.Lock(); defer o.mu.Unlock(); return o.method }

func (o *OP) unlessDown(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o.mu.Lock()
		down := o.down
		o.mu.Unlock()
		if down {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (o *OP) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer": o.URL, "authorization_endpoint": o.URL + "/authorize", "token_endpoint": o.URL + "/token",
		"userinfo_endpoint": o.URL + "/userinfo", "jwks_uri": o.URL + "/jwks",
		"response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"}, "code_challenge_methods_supported": []string{"S256"},
	})
}

func (o *OP) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	code := random()
	o.mu.Lock()
	o.method = q.Get("code_challenge_method")
	o.codes[code] = grant{sub: o.sub, nonce: q.Get("nonce"), challenge: q.Get("code_challenge")}
	o.mu.Unlock()
	to, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	to.RawQuery = url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
	http.Redirect(w, r, to.String(), http.StatusFound) //nolint:gosec // test OP: the client's redirect_uri
}

func (o *OP) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostFormValue("client_id"), r.PostFormValue("client_secret")
	}
	if id != ClientID || secret != ClientSecret {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	var g grant
	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		g, ok = o.codes[r.PostFormValue("code")]
		delete(o.codes, r.PostFormValue("code"))
		sum := sha256.Sum256([]byte(r.PostFormValue("code_verifier")))
		ok = ok && g.challenge == base64.RawURLEncoding.EncodeToString(sum[:])
	case "refresh_token":
		g.sub, ok = o.refreshes[r.PostFormValue("refresh_token")]
		delete(o.refreshes, r.PostFormValue("refresh_token"))
		ok = ok && !o.disabled[g.sub]
	}
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}
	o.issue(w, g)
}

// issue answers the tokens of g; o.mu is held.
func (o *OP) issue(w http.ResponseWriter, g grant) {
	if o.nonce != nil {
		g.nonce = *o.nonce
	}
	now := time.Now()
	claims := map[string]any{"iss": o.URL, "sub": g.sub, "aud": ClientID, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()}
	if g.nonce != "" {
		claims["nonce"] = g.nonce
	}
	for k, v := range o.claims[g.sub] {
		claims[k] = v
	}
	payload, _ := json.Marshal(claims)
	jws, err := o.signer.Sign(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idToken, _ := jws.CompactSerialize()
	access, refresh := random(), random()
	o.access[access], o.refreshes[refresh] = g.sub, g.sub
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 3600,
		"refresh_token": refresh, "id_token": idToken})
}

func (o *OP) userinfo(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	defer o.mu.Unlock()
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	sub, ok := o.access[token]
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	info := map[string]any{"sub": sub}
	for k, v := range o.claims[sub] {
		info[k] = v
	}
	writeJSON(w, http.StatusOK, info)
}

func random() string { return rand.Text() }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
