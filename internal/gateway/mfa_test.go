package gateway

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPromptAny      = "Open https://ui.werbot.test/mfa/c1 (code BCDF-GHJK) or enter TOTP: "
	testPromptWebAuthn = "Open https://ui.werbot.test/mfa/c1 (code BCDF-GHJK): "
)

func TestMFAPromptText(t *testing.T) {
	assert.Equal(t, testPromptAny, promptText("https://ui.werbot.test/mfa/c1", "BCDF-GHJK", true))
	assert.Equal(t, testPromptWebAuthn, promptText("https://ui.werbot.test/mfa/c1", "BCDF-GHJK", false))
}

// mfaOpen opens a terminal on "blocker" asking through a.
func mfaOpen(e *pipeEnv, a *askWith) (*Session, error) {
	r := e.req("blocker", KindTerminal)
	r.MFA = MFAStep{Prompt: a.ask, Session: []byte("sid")}
	return e.p.Open(context.Background(), r)
}

func TestMFAWebAuthnOnlyIgnoresAnswer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, fm := mfaPipe(t, "webauthn")
		fm.status = confirmedFrom(1)
		a := &askWith{answers: []string{"123456"}}
		s, err := mfaOpen(e, a)
		require.NoError(t, err)
		_, codes, _ := fm.calls()
		assert.Empty(t, codes, "a code never meets a webauthn rule")
		assert.Equal(t, []string{testPromptWebAuthn, testPromptWebAuthn}, a.texts, "a typed answer asks again")
		req := e.trackedReq(s.ID)
		assert.Equal(t, int64(0), req.MFAAgeSeconds)
		assert.Equal(t, "webauthn", req.MFAMethod)
		creates, _, _ := fm.calls()
		require.Len(t, creates, 1)
		assert.Equal(t, challengeRequest{UserID: e.m["u1"], ResourceID: e.m["rblocker"], SSHSessionID: []byte("sid"),
			ClientIP: e.req("", "").ClientIP, Binding: "key:SHA256:key", Strength: "webauthn"}, creates[0])
		e.p.abort(s)
	})
}

func TestMFANoticeOffersNoTOTP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, fm := mfaPipe(t, "any")
		fm.status = confirmedFrom(1)
		a := &askWith{}
		r := e.req("blocker", KindExec)
		r.MFA = MFAStep{Prompt: a.ask, Notice: true}
		s, err := e.p.Open(context.Background(), r)
		require.NoError(t, err)
		assert.Equal(t, []string{testPromptWebAuthn}, a.texts, "a prompt that cannot read offers no TOTP")
		e.p.abort(s)
	})
}

func TestMFAConfirmedViaHub(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, fm := mfaPipe(t, "any")
		start := time.Now()
		got := make(chan error, 1)
		go func() {
			s, err := mfaOpen(e, &askWith{})
			if err == nil {
				e.p.abort(s)
			}
			got <- err
		}()
		time.Sleep(500 * time.Millisecond)
		fm.set(func(f *fakeMFA) { f.status = confirmedFrom(1) })
		fm.waits.deliver("c1")
		require.NoError(t, <-got)
		assert.Equal(t, 500*time.Millisecond, time.Since(start), "the hint wakes the wait before the poll")
		_, _, gets := fm.calls()
		assert.Equal(t, 1, gets, "the hint is checked with GetMFAChallenge")
	})
}

func TestMFAConfirmedForOtherChallengeIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, fm := mfaPipe(t, "any")
		start := time.Now()
		got := make(chan error, 1)
		go func() { _, err := mfaOpen(e, &askWith{}); got <- err }()
		time.Sleep(500 * time.Millisecond)
		fm.waits.deliver("c2")
		synctest.Wait()
		_, _, gets := fm.calls()
		assert.Zero(t, gets, "another challenge's confirmation does not wake the wait")
		require.ErrorIs(t, <-got, ErrMFATimeout)
		assert.Equal(t, mfaChallengeTTL, time.Since(start))
		assert.Zero(t, e.f.startCalls)
	})
}

func TestMFAPollFallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, fm := mfaPipe(t, "any")
		fm.status = confirmedFrom(3)
		start := time.Now()
		s, err := mfaOpen(e, &askWith{})
		require.NoError(t, err)
		took := time.Since(start)
		assert.GreaterOrEqual(t, took, 4*time.Second)
		assert.LessOrEqual(t, took, 6*time.Second)
		e.p.abort(s)
	})
}

func TestMFATimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, fm := mfaPipe(t, "any")
		start := time.Now()
		_, err := mfaOpen(e, &askWith{})
		require.ErrorIs(t, err, ErrMFATimeout)
		assert.Equal(t, mfaChallengeTTL, time.Since(start))
		assert.Equal(t, "Permission denied (MFA timeout)", openText(err, "blocker", KindTerminal, nil))

		// a code typed after the challenge expired
		fm.set(func(f *fakeMFA) { f.verify = func(string) (bool, error) { return true, nil } })
		late := func(context.Context, string) (string, error) {
			time.Sleep(mfaChallengeTTL + time.Second)
			return "123456", nil
		}
		r := e.req("blocker", KindTerminal)
		r.MFA.Prompt = late
		_, err = e.p.Open(context.Background(), r)
		require.ErrorIs(t, err, ErrMFATimeout)
		_, codes, _ := fm.calls()
		assert.Empty(t, codes, "a late code is not checked")
		assert.Zero(t, e.f.startCalls)
	})
}

func TestMFATooManyAttemptsText(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, fm := mfaPipe(t, "any")
		fm.verify = func(string) (bool, error) { return false, connect.NewError(connect.CodeResourceExhausted, nil) }
		_, err := mfaOpen(e, &askWith{answers: []string{"123456"}})
		require.ErrorIs(t, err, ErrMFATooManyAttempts)
		assert.Equal(t, "Too many MFA attempts, try again later", openText(err, "blocker", KindTerminal, nil))

		fm.set(func(f *fakeMFA) { f.verify, f.status = nil, func(int) string { return "denied" } })
		_, err = mfaOpen(e, &askWith{})
		require.ErrorIs(t, err, ErrMFATooManyAttempts, "a challenge the server denied")
	})
}

// A session opened after an MFA step carries the factor for its binding: the
// sweeper re-decides it with that factor for as long as it runs.
func TestReevaluateKeepsMFAAge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, fm := mfaPipe(t, "any")
		fm.verify = func(string) (bool, error) { return true, nil }
		startSweeper(t, e.p)
		s, err := mfaOpen(e, &askWith{answers: []string{"123456"}})
		require.NoError(t, err)
		req := e.trackedReq(s.ID)
		assert.Equal(t, "totp", req.MFAMethod)
		assert.Equal(t, int64(0), req.MFAAgeSeconds)
		done := e.runBlocker(t, s)
		time.Sleep(5 * time.Minute)
		assert.Empty(t, e.endedWith(s.ID), "the factor of the session does not age out")

		deny := `forbid (principal, action, resource == Werbot::Resource::"rblocker");`
		e.f.setPolicy(compileV(t, e.m, "v2", pipePolicies+mfaRuleOn("rblocker", "blocker", "any")+deny,
			res{"rfake", "fake", "fake"}, res{"rblocker", "blocker", "blocker"}))
		<-done
		assert.Equal(t, []string{"access_revoked"}, e.endedWith(s.ID))
	})
}

// An agent never meets an MFA rule of its sponsor: a plain deny, no challenge.
func TestAgentOpenNeverGetsMFAChallenge(t *testing.T) {
	e, fm := mfaPipe(t, "any")
	e.f.user.Agent = &Agent{ID: uuid.New(), RunID: uuid.New(), AuthAt: time.Now()}
	_, err := mfaOpen(e, &askWith{answers: []string{"123456"}})
	require.ErrorIs(t, err, ErrDenied)
	assert.Equal(t, "Access denied", openText(err, "blocker", KindTerminal, nil))
	requireNoChallenge(t, fm)
}

// A web terminal cannot ask: it closes with mfa_required.
func TestWebFrontMFARequired(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		fm := withMFA(w.p)
		w.f.setPolicy(compileTest(t, w.m, pipePolicies+mfaRuleOn("recho", "echo", "any"), nil, res{"recho", "echo", "echo"}))
		st := w.open(t, terminalTicket("echo"))
		assert.Equal(t, "mfa_required", st.closedWith(t))
		assert.Equal(t, "MFA required, reconnect to confirm\r\n", st.output())
		assert.Zero(t, w.startCalls())
		requireNoChallenge(t, fm)
		assert.Equal(t, "mfa_required", streamCloseReason(ErrMFARequired))
	})
}
