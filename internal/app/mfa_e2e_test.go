//go:build unix

package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/identity/webauthntest"
)

var errStop = errors.New("test stops answering")

// confirm confirms challenge id in the browser session c with passkey a.
func confirm(c mfaClients, a *webauthntest.Authenticator, id string) error {
	ctx := context.Background()
	ch, err := c.chals.GetChallenge(ctx, connect.NewRequest(&werbotv1.GetChallengeRequest{Id: id}))
	if err != nil {
		return err
	}
	_, err = c.chals.ConfirmChallenge(ctx, connect.NewRequest(&werbotv1.ConfirmChallengeRequest{Id: id,
		WebauthnAssertion: a.Get(ch.Msg.GetWebauthnOptions())}))
	return err
}

// echoHi runs "echo hi" on the open login.
func echoHi(t *testing.T, l *kiLogin) {
	t.Helper()
	require.NoError(t, l.err, "banner: %q", l.banner)
	out, err := run(l.client, "echo hi")
	require.NoError(t, err)
	assert.Equal(t, "hi\n", out)
}

// The SSH prompt names the confirmation page and code; the page shows the
// same login (code, gateway host, client IP, resource), a passkey confirms
// it and an empty answer opens the login within 3s.
func TestE2ESSHMFAApprovePage(t *testing.T) {
	m := mfaSSH(t)
	passkey := enrollPasskey(t, m.web, m.webURL)
	var host string
	require.NoError(t, m.pool.QueryRow(context.Background(), "SELECT hostname FROM gateway_instance").Scan(&host))

	var answered time.Time
	l := m.kiDial(t, m.key, func(id, prompt string) (string, error) {
		ch, err := m.web.chals.GetChallenge(context.Background(), connect.NewRequest(&werbotv1.GetChallengeRequest{Id: id}))
		require.NoError(t, err)
		assert.Equal(t, "Open "+m.webURL+"/mfa/"+id+" (code "+ch.Msg.GetCode()+") or enter TOTP: ", prompt)
		assert.Equal(t, []string{host, "127.0.0.1", "srv", "any"},
			[]string{ch.Msg.GetHost(), ch.Msg.GetClientIp(), ch.Msg.GetResource(), ch.Msg.GetStrength()})
		require.NoError(t, confirm(m.web, passkey, id))
		answered = time.Now()
		return "", nil
	})
	assert.Less(t, time.Since(answered), 3*time.Second, "login after the confirmation")
	assert.Equal(t, 1, l.prompts)
	echoHi(t, l)
}

// A TOTP code typed at the prompt opens the login; the same code at another
// connection in the same step is refused and the prompt asks again.
func TestE2ESSHMFATOTP(t *testing.T) {
	m := mfaSSH(t)
	code := totpAt(t, enrollTOTP(t, m.web), 0)
	echoHi(t, m.kiDial(t, m.key, func(string, string) (string, error) { return code, nil }))

	other := m.addKey(t) // another binding: no MFA to reuse
	l := m.kiDial(t, other, func(string, string) (string, error) { return code, nil })
	require.Error(t, l.err)
	assert.Greater(t, l.prompts, 1, "a spent code asks again")
	assert.Equal(t, 1, m.count(t, "SELECT count(*) FROM mfa_verification"), "only the first login is verified")
}

// A second login with the same key within the rule's 60s needs no second
// factor; past it the prompt comes again.
func TestE2EMFAReuseWithinWindow(t *testing.T) {
	m := mfaSSH(t)
	code := totpAt(t, enrollTOTP(t, m.web), 0)
	echoHi(t, m.kiDial(t, m.key, func(string, string) (string, error) { return code, nil }))
	age := func(d time.Duration) {
		_, err := m.pool.Exec(context.Background(), "UPDATE mfa_verification SET verified_at = verified_at - make_interval(secs => $1)", d.Seconds())
		require.NoError(t, err)
	}

	age(5 * time.Second)
	c, _, err := m.dial(m.gwSSH, "alice+srv", m.key) // public key only
	require.NoError(t, err, "the factor of 5s ago is reused")
	out, err := run(c, "echo hi")
	require.NoError(t, err)
	assert.Equal(t, "hi\n", out)
	require.NoError(t, c.Close())

	age(61 * time.Second)
	l := m.kiDial(t, m.key, func(string, string) (string, error) { return "", errStop })
	require.Error(t, l.err)
	assert.Equal(t, 1, l.prompts, "past the window the factor is asked again")
}

// A challenge that expires while the login waits for the browser refuses
// the login with the MFA timeout text.
func TestE2EMFATimeout(t *testing.T) {
	m := mfaSSH(t)
	l := m.kiDial(t, m.key, func(id, _ string) (string, error) {
		m.expire(t, id)
		return "", nil
	})
	require.Error(t, l.err)
	assert.Equal(t, "Permission denied (MFA timeout)\r\n", l.banner)
}

// Another member cannot confirm alice's challenge, even with a passkey
// assertion over its options: NotFound, and alice's login stays closed.
func TestE2EChallengeOfOtherUserRejected(t *testing.T) {
	m := mfaSSH(t)
	enrollPasskey(t, m.web, m.webURL)
	m.addMember(t, "dave")
	dave := login(t, m.url, "dave@example.com")
	daveKey := enrollPasskey(t, dave, m.webURL)

	l := m.kiDial(t, m.key, func(id, _ string) (string, error) {
		ch, err := m.web.chals.GetChallenge(context.Background(), connect.NewRequest(&werbotv1.GetChallengeRequest{Id: id}))
		require.NoError(t, err)
		_, err = dave.chals.GetChallenge(context.Background(), connect.NewRequest(&werbotv1.GetChallengeRequest{Id: id}))
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "GetChallenge of another user")
		_, err = dave.chals.ConfirmChallenge(context.Background(), connect.NewRequest(&werbotv1.ConfirmChallengeRequest{Id: id,
			WebauthnAssertion: daveKey.Get(ch.Msg.GetWebauthnOptions())}))
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "ConfirmChallenge of another user")
		assert.Equal(t, 1, m.count(t, "SELECT count(*) FROM mfa_challenge WHERE id = $1 AND status = 'pending'", id))
		m.expire(t, id) // end the wait: nobody confirmed it
		return "", nil
	})
	require.Error(t, l.err)
	assert.Equal(t, "Permission denied (MFA timeout)\r\n", l.banner)
	assert.Zero(t, m.count(t, "SELECT count(*) FROM mfa_verification"))
}

// A confirmation through another replica reaches the waiting login over the
// stream of the replica the instance is connected to, well before the 2s poll.
func TestE2EMFAConfirmedAcrossReplicas(t *testing.T) {
	m := mfaSSH(t)
	passkey := enrollPasskey(t, m.web, m.webURL)
	httpB := freeAddr(t)
	b := startRun(t, []string{"server"}, env(map[string]string{
		"WERBOT_DATABASE_URL": m.pool.Config().ConnString(), "WERBOT_HTTP_ADDR": httpB, "WERBOT_PUBLIC_URL": "http://" + httpB,
		"WERBOT_WEB_URL": m.webURL, "WERBOT_SECRET_KEY": testSecretKey, "WERBOT_GATEWAY_ADDR": freeAddr(t),
		"WERBOT_DATA_DIR": t.TempDir(), "WERBOT_ENV": "development",
	})).healthy(t)
	viaB := newMFAClients(m.web.hc, b.api) // the session cookie is the same on both replicas

	confirmed := make(chan time.Time, 1)
	l := m.kiDial(t, m.key, func(id, _ string) (string, error) {
		go func() { // the login waits for the browser from the empty answer on
			err := confirm(viaB, passkey, id)
			assert.NoError(t, err)
			confirmed <- time.Now()
		}()
		return "", nil
	})
	opened := time.Now()
	echoHi(t, l)
	select {
	case at := <-confirmed:
		assert.Less(t, opened.Sub(at), 1500*time.Millisecond, "the stream brought the confirmation, not the poll")
	case <-time.After(5 * time.Second):
		t.Fatal("the confirmation did not return")
	}
}
