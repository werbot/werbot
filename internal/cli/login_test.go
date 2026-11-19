package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginRequiresServer(t *testing.T) {
	h := newHarness(t)
	h.logout(t)
	assert.EqualError(t, h.run("login"), "--server is required")
}

// --server wins over WERBOT_SERVER, which wins over the last login.
func TestLoginServerSources(t *testing.T) {
	h := newHarness(t)
	_ = h.run("login") // the opener cancels the flow once it has the URL
	assert.Contains(t, h.opened, h.srv.URL+"/oauth/authorize?", "current")

	h.vars["WERBOT_SERVER"] = "http://env.example"
	_ = h.run("login")
	assert.Contains(t, h.opened, "http://env.example/oauth/authorize?", "env")

	_ = h.run("login", "--server", "http://flag.example")
	assert.Contains(t, h.opened, "http://flag.example/oauth/authorize?", "flag")
}

func TestStatus(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.run("status"))
	for _, want := range []string{"Server: " + h.srv.URL, "User: alice", "Organization: Acme", "Token expires: ", "Certificate: valid until "} {
		assert.Contains(t, h.out.String(), want)
	}
}

func TestStatusNoCertificate(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, removeCert(h))
	require.NoError(t, h.run("status"))
	assert.Contains(t, h.out.String(), "Certificate: none")
}

func TestStatusNotLoggedIn(t *testing.T) {
	h := newHarness(t)
	h.logout(t)
	assert.EqualError(t, h.run("status"), "not logged in; run werbot login --server <url>")
}
