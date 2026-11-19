package clientauth

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// browse follows the authorize URL like a browser whose user approves.
func browse(t *testing.T) func(string) error {
	return func(u string) error {
		res, err := http.Get(u) //nolint:gosec,noctx // test URL
		if err != nil {
			return err
		}
		_ = res.Body.Close()
		assert.Equal(t, http.StatusOK, res.StatusCode)
		return nil
	}
}

// login runs the loopback flow against as in a fresh home.
func login(t *testing.T, as *fakeAS) (string, *Profile) {
	t.Helper()
	home := t.TempDir()
	p, err := Login(context.Background(), home, as.srv.URL, LoginOptions{OpenBrowser: browse(t)})
	require.NoError(t, err)
	return home, p
}

func TestLoginLoopback(t *testing.T) {
	as := newFakeAS(t)
	home := t.TempDir()
	var out bytes.Buffer
	p, err := Login(context.Background(), home, as.srv.URL+"/", LoginOptions{Out: &out, OpenBrowser: func(u string) error {
		go func() { _ = browse(t)(u) }()
		return errors.New("no browser")
	}})
	require.NoError(t, err)

	assert.Equal(t, as.srv.URL, p.Server)
	assert.Equal(t, "alice", p.Login)
	assert.Equal(t, as.orgID, p.OrgID)
	assert.Equal(t, as.userID, p.UserID)
	assert.Equal(t, "access-1", p.AccessToken)
	assert.WithinDuration(t, time.Now().Add(10*time.Minute), p.AccessExpiry, 5*time.Second)

	q := as.authorizeQuery
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, "openid offline_access", q.Get("scope"))
	assert.Equal(t, "werbot-cli", q.Get("client_id"))
	assert.Equal(t, "code", q.Get("response_type"))
	host, _ := os.Hostname()
	assert.Equal(t, host, q.Get("werbot_hostname"))
	assert.Regexp(t, `^http://127\.0\.0\.1:\d+/callback$`, q.Get("redirect_uri"))
	assert.Len(t, q.Get("state"), 43) // 32 random bytes, base64url

	assert.Regexp(t, regexp.MustCompile(`(?m)^Opening the browser to log in: `+regexp.QuoteMeta(as.srv.URL)+`/oauth/authorize\?`), out.String())
	assert.Contains(t, out.String(), "Open this URL in a browser: "+as.srv.URL+"/oauth/authorize?")

	dir := Dir(home, as.srv.URL)
	assertMode(t, dir, 0o700)
	assertMode(t, filepath.Join(dir, "profile.json"), 0o600)
	cur, err := os.ReadFile(filepath.Join(home, "current")) //nolint:gosec // test temp directory
	require.NoError(t, err)
	assert.Equal(t, as.srv.URL, string(cur))
	saved, err := Current(home, env(nil))
	require.NoError(t, err)
	assert.Equal(t, p.RefreshToken, saved.RefreshToken)
}

func TestLoginLoopbackWrongState(t *testing.T) {
	as := newFakeAS(t)
	_, err := Login(context.Background(), t.TempDir(), as.srv.URL, LoginOptions{OpenBrowser: func(u string) error {
		au, err := url.Parse(u)
		require.NoError(t, err)
		forged := au.Query().Get("redirect_uri") + "?" + url.Values{"code": {"code-1"}, "state": {"forged"}}.Encode()
		res, err := http.Get(forged) //nolint:gosec,noctx // test URL
		require.NoError(t, err)
		_ = res.Body.Close()
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
		return browse(t)(u) // the login keeps waiting for the right state
	}})
	require.NoError(t, err)
}

func TestLoginLoopbackError(t *testing.T) {
	as := newFakeAS(t)
	as.authorizeErr = "interaction_required"
	_, err := Login(context.Background(), t.TempDir(), as.srv.URL, LoginOptions{OpenBrowser: browse(t)})
	require.EqualError(t, err, "login failed: interaction_required")
}

func TestLoginLoopbackCancel(t *testing.T) {
	as := newFakeAS(t)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := Login(ctx, t.TempDir(), as.srv.URL, LoginOptions{OpenBrowser: func(string) error { cancel(); return nil }})
	require.ErrorIs(t, err, context.Canceled)
}

// recordWaits swaps the device-poll wait hook (Ruling G13).
func recordWaits(t *testing.T) *[]time.Duration {
	var waits []time.Duration
	prev := wait
	wait = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return ctx.Err()
	}
	t.Cleanup(func() { wait = prev })
	return &waits
}

func TestLoginDevice(t *testing.T) {
	as := newFakeAS(t)
	as.deviceReplies = []string{"authorization_pending", "slow_down", ""}
	waits := recordWaits(t)
	var out bytes.Buffer
	home := t.TempDir()
	p, err := Login(context.Background(), home, as.srv.URL, LoginOptions{Device: true, Out: &out})
	require.NoError(t, err)
	assert.Equal(t, "alice", p.Login)
	assert.Equal(t, []time.Duration{5 * time.Second, 10 * time.Second}, *waits)
	assert.Contains(t, out.String(), "To log in, open "+as.srv.URL+"/device and enter code BCDF-GHJK\n")
	assert.Contains(t, out.String(), as.srv.URL+"/device?user_code=BCDF-GHJK")
	assert.Equal(t, "openid offline_access", as.authorizeQuery.Get("scope"))
	assert.NotEmpty(t, as.authorizeQuery.Get("werbot_hostname"))
	assert.FileExists(t, filepath.Join(Dir(home, as.srv.URL), "profile.json"))
}

func TestLoginDeviceDeniedAndExpired(t *testing.T) {
	for reply, want := range map[string]string{ //nolint:gosec // OAuth error codes, no credential
		"access_denied": "login denied",
		"expired_token": "login code expired; run werbot login again",
	} {
		as := newFakeAS(t)
		as.deviceReplies = []string{"authorization_pending", reply}
		recordWaits(t)
		_, err := Login(context.Background(), t.TempDir(), as.srv.URL, LoginOptions{Device: true})
		require.EqualError(t, err, want)
	}
}

func TestLoginOnlyTalksToServer(t *testing.T) {
	as := newFakeAS(t)
	login(t, as)
	as.deviceReplies = []string{""}
	recordWaits(t)
	_, err := Login(context.Background(), t.TempDir(), as.srv.URL, LoginOptions{Device: true})
	require.NoError(t, err)
	assert.Zero(t, as.count("/.well-known/openid-configuration"))
	as.mu.Lock()
	defer as.mu.Unlock()
	for path := range as.calls {
		assert.Contains(t, []string{"/oauth/authorize", "/oauth/token", "/oauth/device_authorization", "/werbot.v1.AuthService/GetSession"}, path)
	}
}
