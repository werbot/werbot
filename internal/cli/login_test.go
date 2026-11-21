package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/sshca"
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

// status renews a certificate with less than 10 minutes left.
func TestStatusRenewsCertificate(t *testing.T) {
	h := newHarness(t)
	h.plantCert(t, 5*time.Minute)
	require.NoError(t, h.run("status"))
	assert.Equal(t, 1, h.issued)
	until := time.Now().Add(sshca.UserCertTTL).UTC().Truncate(time.Second)
	assert.Regexp(t, `Certificate: valid until `+until.Format("2006-01-02T15"), h.out.String())
}

// Without a certificate and with renewal refused, status still prints and returns the error.
func TestStatusNoCertificate(t *testing.T) {
	h := newHarness(t)
	h.certFail = true
	require.NoError(t, removeCert(h))
	assert.EqualError(t, h.run("status"), "issue a certificate: unavailable")
	assert.Contains(t, h.out.String(), "Certificate: none")
}

func TestStatusNotLoggedIn(t *testing.T) {
	h := newHarness(t)
	h.logout(t)
	assert.EqualError(t, h.run("status"), "not logged in; run werbot login --server <url>")
}
