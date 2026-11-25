package webauthntest_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/webauthntest"
)

type user struct{ creds []webauthn.Credential }

func (user) WebAuthnID() []byte                           { return bytes.Repeat([]byte{1}, 16) }
func (user) WebAuthnName() string                         { return "alice@example.com" }
func (user) WebAuthnDisplayName() string                  { return "alice@example.com" }
func (u user) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func login(t *testing.T, wa *webauthn.WebAuthn, u user, a *webauthntest.Authenticator) *webauthn.Credential {
	t.Helper()
	assertion, session, err := wa.BeginLogin(u)
	require.NoError(t, err)
	opts, err := json.Marshal(assertion)
	require.NoError(t, err)
	parsed, err := protocol.ParseCredentialRequestResponseBytes(a.Get(opts))
	require.NoError(t, err)
	cred, err := wa.ValidateLogin(u, *session, parsed)
	require.NoError(t, err)
	return cred
}

func TestAuthenticatorRoundTrip(t *testing.T) {
	wa, err := webauthn.New(&webauthn.Config{RPID: "ui.werbot.test", RPDisplayName: "Werbot", RPOrigins: []string{"https://ui.werbot.test"}})
	require.NoError(t, err)
	for _, be := range []bool{false, true} {
		a := webauthntest.New(t, "https://ui.werbot.test")
		a.BackupEligible = be
		creation, session, err := wa.BeginRegistration(user{})
		require.NoError(t, err)
		opts, err := json.Marshal(creation)
		require.NoError(t, err)
		parsed, err := protocol.ParseCredentialCreationResponseBytes(a.Create(opts))
		require.NoError(t, err)
		cred, err := wa.CreateCredential(user{}, *session, parsed)
		require.NoError(t, err)
		assert.Equal(t, "none", cred.AttestationFormat)
		assert.Equal(t, be, cred.Flags.BackupEligible)

		u := user{creds: []webauthn.Credential{*cred}}
		got := login(t, wa, u, a)
		assert.Equal(t, cred.ID, got.ID)
		assert.Equal(t, uint32(1), got.Authenticator.SignCount)
		assert.False(t, got.Authenticator.CloneWarning)

		u.creds[0].Authenticator.SignCount = 5
		a.SetSignCount(5)
		assert.True(t, login(t, wa, u, a).Authenticator.CloneWarning, "a count not above the stored one")
	}
}
