package connectcli

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestEncryptedKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "id")
	writeKey(t, path, "secret")
	_, _, err := authMethod(path)
	assert.EqualError(t, err, "key "+path+" is encrypted; load it into ssh-agent")
}

// Without -i and SSH_AUTH_SOCK the key is ~/.ssh/id_ed25519.
func TestDefaultKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("SSH_AUTH_SOCK", "")
	_, _, err := authMethod("")
	assert.ErrorContains(t, err, filepath.Join(home, ".ssh", "id_ed25519"))

	key := writeKey(t, filepath.Join(home, ".ssh", "id_ed25519"), "")
	port := freePortNum(t)
	f := startFake(t, key.PublicKey(), tcpInfo)
	r := startRun(t, Config{Gateway: f.addr, Login: "alice", KnownHosts: filepath.Join(home, "kh"),
		Specs: []Spec{{LocalPort: port, Resource: "pg"}}})
	r.ready(t, "pg", port, "tcp")
	assert.NoError(t, r.stop(t))
}

// Without -i the ssh-agent at SSH_AUTH_SOCK signs.
func TestAgentKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no unix sockets on the Windows CI")
	}
	dir, err := os.MkdirTemp("", "ag") // short: unix socket paths are limited to ~100 bytes
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	ln, err := net.Listen("unix", filepath.Join(dir, "s"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ring := agent.NewKeyring()
	require.NoError(t, ring.Add(agent.AddedKey{PrivateKey: priv}))
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { _ = agent.ServeAgent(ring, c); _ = c.Close() }()
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(dir, "s"))

	signer, err := gossh.NewSignerFromKey(priv)
	require.NoError(t, err)
	port := freePortNum(t)
	f := startFake(t, signer.PublicKey(), tcpInfo)
	r := startRun(t, Config{Gateway: f.addr, Login: "alice", KnownHosts: filepath.Join(t.TempDir(), "kh"),
		Specs: []Spec{{LocalPort: port, Resource: "pg"}}})
	r.ready(t, "pg", port, "tcp")
	assert.NoError(t, r.stop(t))
}
