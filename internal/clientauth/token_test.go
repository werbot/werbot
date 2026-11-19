package clientauth

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// expireSoon moves the access token 30 s from expiry and saves the profile.
func expireSoon(t *testing.T, home string, p *Profile) {
	t.Helper()
	p.AccessExpiry = time.Now().Add(30 * time.Second)
	require.NoError(t, p.Save(home))
}

func TestTokenRefresh(t *testing.T) {
	as := newFakeAS(t)
	home, p := login(t, as)

	p.AccessExpiry = time.Now().Add(5 * time.Minute)
	tok, err := p.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "access-1", tok)
	assert.Zero(t, as.count("refresh"))

	expireSoon(t, home, p)
	tok, err = p.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok)
	assert.Equal(t, 1, as.count("refresh"))
	saved, err := Current(home, env(nil))
	require.NoError(t, err)
	assert.Equal(t, "refresh-2", saved.RefreshToken)
	assert.Equal(t, "access-2", saved.AccessToken)
	assertMode(t, filepath.Join(Dir(home, p.Server), "profile.json"), 0o600)
	assert.NoFileExists(t, filepath.Join(Dir(home, p.Server), ".lock"))
}

func TestConcurrentTokenRefreshRotatesOnce(t *testing.T) {
	as := newFakeAS(t)
	home, p1 := login(t, as)
	expireSoon(t, home, p1)
	p2, err := Current(home, env(nil))
	require.NoError(t, err)
	as.refreshDelay = 100 * time.Millisecond

	var wg sync.WaitGroup
	toks := make([]string, 2)
	errs := make([]error, 2)
	for i, p := range []*Profile{p1, p2} {
		wg.Go(func() { toks[i], errs[i] = p.Token(context.Background()) })
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.Equal(t, 1, as.count("refresh"))
	assert.Equal(t, toks[0], toks[1])
	assert.Equal(t, "access-2", toks[0])
}

func TestStaleLockIsTaken(t *testing.T) {
	as := newFakeAS(t)
	home, p := login(t, as)
	expireSoon(t, home, p)
	lock := filepath.Join(Dir(home, p.Server), ".lock")
	require.NoError(t, os.WriteFile(lock, nil, 0o600))
	old := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(lock, old, old))
	tok, err := p.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok)
}

func TestTokenInvalidGrant(t *testing.T) {
	as := newFakeAS(t)
	home, p := login(t, as)
	expireSoon(t, home, p)
	as.refreshErr = "invalid_grant"
	_, err := p.Token(context.Background())
	require.ErrorIs(t, err, ErrLoginRequired)
	assert.FileExists(t, filepath.Join(Dir(home, p.Server), "profile.json")) // kept (Ruling R9)

	// Through the API client the cause stays visible to the CLI.
	_, err = werbotv1connect.NewAuthServiceClient(p.HTTPClient(), p.Server).
		GetSession(context.Background(), connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.ErrorIs(t, err, ErrLoginRequired)
}

func TestHTTPClientBearer(t *testing.T) {
	as := newFakeAS(t)
	home, p := login(t, as)
	client := werbotv1connect.NewAuthServiceClient(p.HTTPClient(), p.Server)
	_, err := client.GetSession(context.Background(), connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "access-1", as.lastBearer)

	expireSoon(t, home, p)
	_, err = client.GetSession(context.Background(), connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "access-2", as.lastBearer)
}
