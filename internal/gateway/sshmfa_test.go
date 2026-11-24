package gateway

import (
	"log/slog"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// loginCtx adds the login to remoteCtx.
type loginCtx struct {
	remoteCtx
	user string
}

func (c loginCtx) User() string { return c.user }

// verifiedEnv is an SSH handler on a pipeline where everyone may connect and
// u1 needs a fresh factor for "blocker"; keys "a" (u1) and "b" (u2) both log in as alice.
type verifiedEnv struct {
	e    *pipeEnv
	fm   *fakeMFA
	h    *sshHandler
	keys map[string]gossh.Signer
}

func newVerifiedEnv(t *testing.T) *verifiedEnv {
	t.Helper()
	e := newPipeEnv(t, 10)
	e.m["u2"] = uuid.New()
	v := &verifiedEnv{e: e, keys: map[string]gossh.Signer{"a": newKey(t), "b": newKey(t)}}
	e.f.policy = compileTest(t, e.m, `permit (principal, action, resource);`+mfaRuleOn("rblocker", "blocker", "any"), nil,
		res{"rblocker", "blocker", "blocker"})
	e.f.byKey = map[string]User{fp(v.keys["a"]): {ID: e.m["u1"], Login: "alice"}, fp(v.keys["b"]): {ID: e.m["u2"], Login: "alice"}}
	e.f.keyErrs = map[string]error{}
	v.fm = withMFA(e.p)
	v.h = &sshHandler{p: e.p, log: slog.New(slog.DiscardHandler)}
	return v
}

// conn is the context of a connection to alice+blocker whose last key auth was for key.
func (v *verifiedEnv) conn(t *testing.T, key string) loginCtx {
	t.Helper()
	ctx := loginCtx{remoteCtx{&fakeConnContext{ctx: t.Context()}}, "alice+blocker"}
	require.True(t, v.h.auth(ctx, v.keys[key].PublicKey()))
	ctx.SetValue(ssh.ContextKeyPublicKey, v.keys[key].PublicKey())
	return ctx
}

// verify runs the verified-key callback of ctx for key.
func (v *verifiedEnv) verify(ctx ssh.Context, key string) (*gossh.Permissions, error) {
	return v.h.verified(ctx).VerifiedPublicKeyCallback(nil, v.keys[key].PublicKey(), &gossh.Permissions{}, "")
}

// The partial success gives the client the challenge's time plus 10 s; one
// that never answers the prompt is closed then, not at once and not later.
func TestMFAPromptUnansweredClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		v := newVerifiedEnv(t)
		ctx := v.conn(t, "a")
		var closed atomic.Bool
		ctx.SetValue(handshakeKey{}, time.AfterFunc(30*time.Second, func() { closed.Store(true) }))
		perms, err := v.verify(ctx, "a")
		ps, ok := err.(*gossh.PartialSuccessError)
		require.True(t, ok, "%v", err)
		assert.Nil(t, perms, "partial success carries no permissions")
		assert.NotNil(t, ps.Next.KeyboardInteractiveCallback)
		assert.Nil(t, ps.Next.PublicKeyCallback)
		assert.Nil(t, ps.Next.PasswordCallback)

		time.Sleep(mfaChallengeTTL + 10*time.Second - time.Millisecond)
		assert.False(t, closed.Load(), "not closed before the challenge's time")
		time.Sleep(time.Millisecond)
		synctest.Wait()
		assert.True(t, closed.Load(), "closed once the challenge's time is over")
		requireNoChallenge(t, v.fm)
	})
}

func TestMFADecisionUsesVerifiedKey(t *testing.T) {
	v := newVerifiedEnv(t)
	// offered key b (u2, no rule), verified key a (u1 under the rule)
	ctx := v.conn(t, "b")
	_, err := v.verify(ctx, "a")
	require.IsType(t, &gossh.PartialSuccessError{}, err, "the decision is the verified key's")
	assert.Equal(t, v.e.m["u1"], handshakeUser(ctx).ID)
	v.e.f.with(func(f *fakeControl) {
		assert.Contains(t, f.authCalls[len(f.authCalls)-1], fp(v.keys["a"]), "the verified key is looked up")
	})
	assert.Equal(t, fp(v.keys["a"]), v.h.request(ctx, "blocker", KindExec, Window{}, "").Identity.Value)

	// offered key a (u1 under the rule), verified key b (u2)
	ctx = v.conn(t, "a")
	perms, err := v.verify(ctx, "b")
	require.NoError(t, err)
	assert.NotNil(t, perms)
	assert.Equal(t, v.e.m["u2"], handshakeUser(ctx).ID)
	requireNoChallenge(t, v.fm)
}

// An agent of a sponsor under an MFA rule is denied plainly: no partial success, no challenge.
func TestAgentNeverGetsMFAChallenge(t *testing.T) {
	v := newVerifiedEnv(t)
	v.e.f.byKey[fp(v.keys["a"])] = User{ID: v.e.m["u1"], Login: "alice", Agent: &Agent{ID: uuid.New(), RunID: uuid.New(), AuthAt: time.Now()}}
	perms, err := v.verify(v.conn(t, "a"), "a")
	require.NoError(t, err)
	assert.NotNil(t, perms)
	requireNoChallenge(t, v.fm)
}
