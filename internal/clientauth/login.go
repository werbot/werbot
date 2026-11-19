package clientauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

const (
	loginTimeout      = 10 * time.Minute
	slowDownStep      = 5 * time.Second
	defaultPollPeriod = 5 * time.Second
)

// LoginOptions selects the login flow and where its prompts go.
type LoginOptions struct {
	Device      bool               // device flow instead of the loopback browser flow
	OpenBrowser func(string) error // nil or failing: the URL is only printed
	Out         io.Writer          // nil: prompts are discarded
}

// wait sleeps between device polls; tests replace it (Ruling G13).
var wait = func(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Login signs in to server, reads who the token belongs to and saves the
// profile as the current one. Only <server>/oauth/* and <server>/werbot.v1.*
// are called; discovery is never read.
func Login(ctx context.Context, home, server string, opts LoginOptions) (*Profile, error) {
	server, err := normalizeServer(server)
	if err != nil {
		return nil, err
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	flow := loginLoopback
	if opts.Device {
		flow = loginDevice
	}
	tok, err := flow(ctx, server, opts)
	if err != nil {
		return nil, err
	}
	p := &Profile{Server: server, home: home}
	p.setTokens(tok)
	sess, err := werbotv1connect.NewAuthServiceClient(p.HTTPClient(), server).
		GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	if err != nil {
		return nil, fmt.Errorf("read the session: %w", err)
	}
	p.Login = sess.Msg.GetUser().GetLogin()
	if p.UserID, err = uuid.Parse(sess.Msg.GetUser().GetId()); err != nil {
		return nil, fmt.Errorf("read the session: user id: %w", err)
	}
	if p.OrgID, err = uuid.Parse(sess.Msg.GetOrganization().GetId()); err != nil {
		return nil, fmt.Errorf("read the session: organization id: %w", err)
	}
	return p, p.Save(home)
}

// random returns 32 random bytes in base64url.
func random() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	return base64.RawURLEncoding.EncodeToString(b)
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

// loginLoopback runs the authorization code flow with PKCE (S256) and a
// redirect to a one-shot listener on 127.0.0.1.
func loginLoopback(ctx context.Context, server string, opts LoginOptions) (tokenResponse, error) {
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return tokenResponse{}, err
	}
	redirect := "http://" + ln.Addr().String() + "/callback"
	state, verifier := random(), random()
	challenge := sha256.Sum256([]byte(verifier))
	authURL := server + "/oauth/authorize?" + url.Values{
		"client_id": {clientID}, "response_type": {"code"}, "scope": {scope}, "redirect_uri": {redirect},
		"state": {state}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"}, "werbot_hostname": {hostname()},
	}.Encode()

	codes := make(chan url.Values, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: callback(state, codes)}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	_, _ = fmt.Fprintf(opts.Out, "Opening the browser to log in: %s\n", authURL)
	if opts.OpenBrowser == nil || opts.OpenBrowser(authURL) != nil {
		_, _ = fmt.Fprintf(opts.Out, "Open this URL in a browser: %s\n", authURL)
	}
	var q url.Values
	select {
	case <-ctx.Done():
		return tokenResponse{}, ctx.Err()
	case q = <-codes:
	}
	if e := q.Get("error"); e != "" {
		return tokenResponse{}, fmt.Errorf("login failed: %s", e)
	}
	return tokenRequest(ctx, server, url.Values{
		"grant_type": {"authorization_code"}, "code": {q.Get("code")}, "redirect_uri": {redirect}, "code_verifier": {verifier},
	})
}

// callback answers 400 to a foreign state and hands the first matching
// answer (code or error) to codes.
func callback(state string, codes chan<- url.Values) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/callback" || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 ||
			(q.Get("code") == "" && q.Get("error") == "") {
			http.Error(w, "invalid login callback", http.StatusBadRequest)
			return
		}
		msg := "Login complete. You can close this window."
		if q.Get("error") != "" {
			msg = "Login failed. You can close this window."
		}
		_, _ = io.WriteString(w, msg)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case codes <- q:
		default: // a second answer for the same state
		}
	})
}

// deviceAuthorization is the RFC 8628 answer.
type deviceAuthorization struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int64  `json:"interval"`
}

// loginDevice runs the device flow: it polls at once, then after every wait.
func loginDevice(ctx context.Context, server string, opts LoginOptions) (tokenResponse, error) {
	var da deviceAuthorization
	if err := postForm(ctx, server+"/oauth/device_authorization", url.Values{
		"client_id": {clientID}, "scope": {scope}, "werbot_hostname": {hostname()},
	}, &da); err != nil {
		return tokenResponse{}, err
	}
	_, _ = fmt.Fprintf(opts.Out, "To log in, open %s and enter code %s\n%s\n", da.VerificationURI, da.UserCode, da.VerificationURIComplete)
	interval := time.Duration(da.Interval) * time.Second
	if interval <= 0 {
		interval = defaultPollPeriod
	}
	for {
		tok, err := tokenRequest(ctx, server, url.Values{
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "device_code": {da.DeviceCode},
		})
		switch oauthCode(err) {
		case "":
			return tok, err
		case "authorization_pending":
		case "slow_down":
			interval += slowDownStep
		case "access_denied":
			return tokenResponse{}, errors.New("login denied")
		case "expired_token":
			return tokenResponse{}, errors.New("login code expired; run werbot login again")
		default:
			return tokenResponse{}, err
		}
		if err := wait(ctx, interval); err != nil {
			return tokenResponse{}, err
		}
	}
}
