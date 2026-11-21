package clientauth

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }

func TestHomeAndConfigVars(t *testing.T) {
	home, err := Home(env(map[string]string{"WERBOT_HOME": "/x"}))
	require.NoError(t, err)
	assert.Equal(t, "/x", home)

	user, err := os.UserHomeDir()
	require.NoError(t, err)
	home, err = Home(env(nil))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(user, ".werbot"), home)

	assert.Equal(t, []string{"WERBOT_SERVER", "WERBOT_HOME"}, ConfigVars())
}

func TestDir(t *testing.T) {
	for server, want := range map[string]string{
		"https://w.example.com":  "w.example.com",
		"http://127.0.0.1:8080":  "127.0.0.1_8080",
		"http://[::1]:8080":      "___1__8080",
		"https://w.example.com/": "w.example.com",
	} {
		assert.Equal(t, filepath.Join("/h", want), Dir("/h", server), server)
	}
}

func TestNormalizeServer(t *testing.T) {
	got, err := normalizeServer("https://w.example.com/")
	require.NoError(t, err)
	assert.Equal(t, "https://w.example.com", got)
	for _, bad := range []string{"", "w.example.com", "ftp://w.example.com", "https://w.example.com/api", "https://w.example.com?x=1", "https://u@w.example.com"} {
		_, err := normalizeServer(bad)
		assert.Error(t, err, bad)
	}
}

func TestCurrent(t *testing.T) {
	home := t.TempDir()
	_, err := Current(home, env(nil))
	require.ErrorIs(t, err, ErrNotLoggedIn)

	a := &Profile{Server: "https://a.example.com", UserID: uuid.New(), Login: "alice", AccessExpiry: time.Now().Add(time.Hour).UTC()}
	b := &Profile{Server: "http://127.0.0.1:8080", UserID: uuid.New(), Login: "bob"}
	require.NoError(t, a.Save(home))
	require.NoError(t, b.Save(home)) // b is now current

	p, err := Current(home, env(nil))
	require.NoError(t, err)
	assert.Equal(t, "bob", p.Login)

	p, err = Current(home, env(map[string]string{"WERBOT_SERVER": "https://a.example.com/"}))
	require.NoError(t, err)
	assert.Equal(t, a.UserID, p.UserID)
	assert.True(t, a.AccessExpiry.Equal(p.AccessExpiry))

	_, err = Current(home, env(map[string]string{"WERBOT_SERVER": "https://c.example.com"}))
	require.ErrorIs(t, err, ErrNotLoggedIn)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return // Windows has no POSIX modes (Ruling R17)
	}
	st, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, want, st.Mode().Perm(), path)
}

func TestLogout(t *testing.T) {
	as := newFakeAS(t)
	home, p := login(t, as)
	refresh := p.RefreshToken
	require.NoError(t, p.Logout(context.Background(), home))
	assert.Equal(t, refresh+" refresh_token", as.revoked)
	assert.NoDirExists(t, Dir(home, p.Server))
	assert.NoFileExists(t, filepath.Join(home, "current"))

	// Server unreachable: the profile is removed locally, the revoke error is returned.
	home, p = login(t, as)
	as.srv.Close()
	err := p.Logout(context.Background(), home)
	require.Error(t, err)
	assert.NoDirExists(t, Dir(home, p.Server))
	assert.NoFileExists(t, filepath.Join(home, "current"))
}

func TestLogoutKeepsOtherCurrent(t *testing.T) {
	as := newFakeAS(t)
	home, p := login(t, as)
	other := &Profile{Server: "https://other.example.com"}
	require.NoError(t, other.Save(home))
	require.NoError(t, p.Logout(context.Background(), home))
	cur, err := Current(home, env(nil))
	require.NoError(t, err)
	assert.Equal(t, other.Server, cur.Server)
}
