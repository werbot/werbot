package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
)

func TestLs(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.run("ls"))
	assert.Regexp(t, `NAME\s+TYPE\s+KINDS\s+GATEWAY\s+ACCESS\ndb\s+ssh\s+terminal,exec,stream\s+gw\.example\.com:2222\s+yes\n`, h.out.String())

	h.out.Reset()
	h.resources = []*werbotv1.AccessibleResource{}
	require.NoError(t, h.run("ls"))
	assert.Equal(t, "No accessible resources\n", h.out.String())
}

// A refused token and a refused refresh both tell the user to log in again.
func TestLoginRequired(t *testing.T) {
	h := newHarness(t)
	h.unauth = true
	for _, args := range [][]string{{"ls"}, {"status"}, {"ssh", "db"}, {"ssh-config"}, {"connect", "db"}} {
		assert.EqualError(t, h.run(args...), clientauth.ErrLoginRequired.Error(), args)
	}

	h = newHarness(t)
	p, err := clientauth.Current(h.home, h.env.Getenv)
	require.NoError(t, err)
	p.AccessExpiry = time.Now().Add(-time.Minute)
	require.NoError(t, p.Save(h.home))
	assert.EqualError(t, h.run("ls"), clientauth.ErrLoginRequired.Error(), "expired access token, refresh refused")
}
