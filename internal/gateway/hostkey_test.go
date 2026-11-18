package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func newHostKey(t *testing.T) gossh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	k, err := gossh.NewPublicKey(pub)
	require.NoError(t, err)
	return k
}

// pinSnapshot applies a snapshot in which "fake" has the pinned fingerprint fp
// (empty: none); without listed, "fake" is not in the snapshot.
func (e *pipeEnv) pinSnapshot(t *testing.T, fp string, listed bool) {
	t.Helper()
	rs := []res{{"rblocker", "blocker", "blocker"}}
	if listed {
		rs = append(rs, res{"rfake", "fake", "fake"})
	}
	s := testSnapshot(t, e.m, pipePolicies, nil, rs...)
	for _, r := range s.GetResources() {
		if r.GetId() == e.m["rfake"].String() {
			r.HostKeySha256 = fp
		}
	}
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	e.f.setPolicy(p)
}

func (e *pipeEnv) pinCalls() (n int) {
	e.f.with(func(f *fakeControl) { n = len(f.pins) })
	return n
}

func TestPinKeySkipsKeyPinnedInSnapshot(t *testing.T) {
	e := newPipeEnv(t, 10)
	key, other := newHostKey(t), newHostKey(t)
	e.pinSnapshot(t, gossh.FingerprintSHA256(key), true)

	s := e.open(t, "fake", KindTerminal)
	require.NoError(t, s.PinKey(key.Type(), key.Marshal()))
	require.NoError(t, e.open(t, "fake", KindTerminal).PinKey(key.Type(), key.Marshal()))
	require.Zero(t, e.pinCalls(), "the server already confirmed this key")

	// the server refuses an alg that is not the key type (invalid_argument)
	e.f.with(func(f *fakeControl) { f.pinErr = errors.New("alg does not match the key") })
	require.Error(t, s.PinKey("rsa-sha2-256", key.Marshal()))
	require.Equal(t, 1, e.pinCalls(), "the pinned key under another alg goes to the server")

	e.f.with(func(f *fakeControl) { f.pinErr = ErrHostKeyMismatch })
	require.ErrorIs(t, s.PinKey(other.Type(), other.Marshal()), ErrHostKeyMismatch)
	require.Equal(t, 2, e.pinCalls(), "a changed key goes to the server, which refuses it")
}

func TestPinKeyAfterClear(t *testing.T) {
	e := newPipeEnv(t, 10)
	key := newHostKey(t)
	e.pinSnapshot(t, gossh.FingerprintSHA256(key), true)
	e.pinSnapshot(t, "", true) // ClearHostKey: the server sends the resource without a pin

	require.NoError(t, e.open(t, "fake", KindTerminal).PinKey(key.Type(), key.Marshal()))
	require.Equal(t, 1, e.pinCalls(), "the next connection pins anew")
}

func TestPinKeyEntryLeavesWithResource(t *testing.T) {
	e := newPipeEnv(t, 10)
	key := newHostKey(t)
	e.pinSnapshot(t, gossh.FingerprintSHA256(key), true)
	e.pinSnapshot(t, "", false)
	_, ok := e.f.Policy().resourceByID(e.m["rfake"])
	require.False(t, ok, "no pin is kept for a resource outside the snapshot")

	e.pinSnapshot(t, "", true) // back in the pool, its pin cleared meanwhile
	require.NoError(t, e.open(t, "fake", KindTerminal).PinKey(key.Type(), key.Marshal()))
	require.Equal(t, 1, e.pinCalls())
}
