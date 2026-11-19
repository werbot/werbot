package clientauth

import (
	"cmp"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// fakeAS stands in for werbot server: the /oauth/* endpoints, a discovery
// document pointing elsewhere, AuthService.GetSession and CertService.IssueUserCert.
type fakeAS struct {
	werbotv1connect.UnimplementedAuthServiceHandler
	werbotv1connect.UnimplementedCertServiceHandler

	srv            *httptest.Server
	ca             ssh.Signer
	userID, orgID  uuid.UUID
	authorizeQuery url.Values

	mu            sync.Mutex
	calls         map[string]int // by path; refresh grants also under "refresh"
	seq           int
	challenge     string
	access        string
	refresh       string
	lastBearer    string
	revoked       string
	authorizeErr  string        // non-empty: /oauth/authorize answers with this error
	refreshErr    string        // non-empty: refresh grants answer with this error
	refreshDelay  time.Duration // widens the race window of concurrent refreshes
	deviceReplies []string      // device grant answers in order; "" issues tokens
	certFail      bool          // IssueUserCert answers Unavailable
	certSkew      time.Duration // server clock ahead of the client
}

func newFakeAS(t *testing.T) *fakeAS {
	t.Helper()
	_, caKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	ca, err := ssh.NewSignerFromKey(caKey)
	require.NoError(t, err)
	as := &fakeAS{ca: ca, userID: uuid.New(), orgID: uuid.New(), calls: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/authorize", as.authorize)
	mux.HandleFunc("/oauth/token", as.token)
	mux.HandleFunc("/oauth/device_authorization", as.deviceAuthorization)
	mux.HandleFunc("/oauth/revoke", as.revoke)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"issuer": "http://api.internal", "token_endpoint": "http://api.internal/oauth/token"}) //nolint:gosec // test discovery document, no credential
	})
	mux.Handle(werbotv1connect.NewAuthServiceHandler(as))
	mux.Handle(werbotv1connect.NewCertServiceHandler(as))
	as.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		as.mu.Lock()
		as.calls[r.URL.Path]++
		as.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(as.srv.Close)
	return as
}

func (as *fakeAS) count(key string) int {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.calls[key]
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// authorize plays the browser that approved the consent screen.
func (as *fakeAS) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	as.mu.Lock()
	as.authorizeQuery, as.challenge = q, q.Get("code_challenge")
	answer := url.Values{"state": {q.Get("state")}}
	if as.authorizeErr != "" {
		answer.Set("error", as.authorizeErr)
	} else {
		answer.Set("code", "code-1")
	}
	as.mu.Unlock()
	http.Redirect(w, r, q.Get("redirect_uri")+"?"+answer.Encode(), http.StatusFound) //nolint:gosec // test AS redirects to the loopback listener under test
}

// issue rotates the token pair; the caller holds mu.
func (as *fakeAS) issue(w http.ResponseWriter) {
	as.seq++
	as.access, as.refresh = fmt.Sprintf("access-%d", as.seq), fmt.Sprintf("refresh-%d", as.seq)
	writeJSON(w, http.StatusOK, map[string]any{"access_token": as.access, "refresh_token": as.refresh, "token_type": "Bearer", "expires_in": 600})
}

func oauthErr(w http.ResponseWriter, code string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": code})
}

func (as *fakeAS) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if r.PostForm.Get("client_id") != "werbot-cli" {
		oauthErr(w, "invalid_client")
		return
	}
	as.mu.Lock()
	defer as.mu.Unlock()
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if r.PostForm.Get("code") != "code-1" || base64.RawURLEncoding.EncodeToString(sum[:]) != as.challenge ||
			r.PostForm.Get("redirect_uri") != as.authorizeQuery.Get("redirect_uri") {
			oauthErr(w, "invalid_grant")
			return
		}
		as.issue(w)
	case "refresh_token":
		as.calls["refresh"]++
		time.Sleep(as.refreshDelay)
		if as.refreshErr != "" || r.PostForm.Get("refresh_token") != as.refresh {
			oauthErr(w, cmp.Or(as.refreshErr, "invalid_grant"))
			return
		}
		as.issue(w)
	case "urn:ietf:params:oauth:grant-type:device_code":
		if r.PostForm.Get("device_code") != "dev-1" || len(as.deviceReplies) == 0 {
			oauthErr(w, "invalid_grant")
			return
		}
		reply := as.deviceReplies[0]
		as.deviceReplies = as.deviceReplies[1:]
		if reply != "" {
			oauthErr(w, reply)
			return
		}
		as.issue(w)
	default:
		oauthErr(w, "unsupported_grant_type")
	}
}

func (as *fakeAS) deviceAuthorization(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	as.mu.Lock()
	as.authorizeQuery = r.PostForm
	as.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code": "dev-1", "user_code": "BCDF-GHJK", "expires_in": 600, "interval": 5,
		"verification_uri": as.srv.URL + "/device", "verification_uri_complete": as.srv.URL + "/device?user_code=BCDF-GHJK",
	})
}

func (as *fakeAS) revoke(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	as.mu.Lock()
	as.revoked = r.PostForm.Get("token") + " " + r.PostForm.Get("token_type_hint")
	as.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

// bearer checks the Authorization header against the live access token.
func (as *fakeAS) bearer(h http.Header) error {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.lastBearer = strings.TrimPrefix(h.Get("Authorization"), "Bearer ")
	if as.access == "" || as.lastBearer != as.access {
		return connect.NewError(connect.CodeUnauthenticated, nil)
	}
	return nil
}

func (as *fakeAS) GetSession(_ context.Context, req *connect.Request[werbotv1.GetSessionRequest]) (*connect.Response[werbotv1.GetSessionResponse], error) {
	if err := as.bearer(req.Header()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.GetSessionResponse{
		User:         &werbotv1.User{Id: as.userID.String(), Login: "alice"},
		Organization: &werbotv1.Organization{Id: as.orgID.String()},
	}), nil
}

func (as *fakeAS) IssueUserCert(_ context.Context, req *connect.Request[werbotv1.IssueUserCertRequest]) (*connect.Response[werbotv1.IssueUserCertResponse], error) {
	if err := as.bearer(req.Header()); err != nil {
		return nil, err
	}
	as.mu.Lock()
	fail, skew := as.certFail, as.certSkew
	as.mu.Unlock()
	if fail {
		return nil, connect.NewError(connect.CodeUnavailable, nil)
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.Msg.PublicKey))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	cert, err := as.sign(pub, time.Now().Add(skew), sshca.UserCertTTL)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.IssueUserCertResponse{Certificate: string(ssh.MarshalAuthorizedKey(cert))}), nil
}

func (as *fakeAS) sign(pub ssh.PublicKey, now time.Time, ttl time.Duration) (*ssh.Certificate, error) {
	return sshca.Sign(as.ca, pub, sshca.Identity{OrgID: as.orgID, UserID: as.userID, CLISessionID: uuid.New(), Login: "alice"}, now, ttl)
}
