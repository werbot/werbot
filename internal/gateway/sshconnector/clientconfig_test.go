package sshconnector

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/sshtest"
)

func acceptKey(string, []byte) error { return nil }

func TestClientConfigPassword(t *testing.T) {
	cfg, err := clientConfig("root", gateway.Secret{Kind: "password", Value: []byte("pw")}, acceptKey, time.Second)
	require.NoError(t, err)
	require.Equal(t, "root", cfg.User)
	require.Len(t, cfg.Auth, 1)
	require.Equal(t, time.Second, cfg.Timeout)

	target := sshtest.StartTarget(t, "pw", nil)
	client, err := gossh.Dial("tcp", target.Addr, cfg)
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestClientConfigPrivateKey(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := gossh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	sshPub, err := gossh.NewPublicKey(pub)
	require.NoError(t, err)

	cfg, err := clientConfig("root", gateway.Secret{Kind: "private_key", Value: pem.EncodeToMemory(block)}, acceptKey, time.Second)
	require.NoError(t, err)
	require.Len(t, cfg.Auth, 1)

	// the target accepts only this key: the publickey method is the one that works
	target := sshtest.StartTarget(t, "", sshPub)
	client, err := gossh.Dial("tcp", target.Addr, cfg)
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestClientConfigInvalidKey(t *testing.T) {
	_, err := clientConfig("root", gateway.Secret{Kind: "private_key", Value: []byte("garbage")}, acceptKey, time.Second)
	require.ErrorContains(t, err, "parse private key")
	require.NotContains(t, err.Error(), "garbage")
}

func TestClientConfigUnsupportedKind(t *testing.T) {
	_, err := clientConfig("root", gateway.Secret{Kind: "token", Value: []byte("t")}, acceptKey, time.Second)
	require.ErrorContains(t, err, `credential kind "token" is not supported`)
}

func TestClientConfigEmptySecret(t *testing.T) {
	_, err := clientConfig("root", gateway.Secret{Kind: "password"}, acceptKey, time.Second)
	require.ErrorContains(t, err, "empty credential")
}
