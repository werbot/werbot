package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAskpass(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	good := map[string]string{"SSH_ASKPASS": "/bin/werbot", "SSH_ASKPASS_REQUIRE": "force"} //nolint:gosec // not a credential
	assert.True(t, IsAskpass([]string{"Password: "}, env(good), "/bin/werbot"))
	assert.False(t, IsAskpass(nil, env(good), "/bin/werbot"), "no prompt")
	assert.False(t, IsAskpass([]string{"status", "x"}, env(good), "/bin/werbot"), "a command line")
	assert.False(t, IsAskpass([]string{"p"}, env(map[string]string{"SSH_ASKPASS": "/bin/werbot"}), "/bin/werbot"), "not forced") //nolint:gosec // not a credential
	assert.False(t, IsAskpass([]string{"p"}, env(good), "/other/werbot"), "another helper")
	assert.False(t, IsAskpass([]string{"p"}, env(map[string]string{"SSH_ASKPASS_REQUIRE": "force"}), "/bin/werbot"), "no helper")
}

type fakeTTY struct {
	io.Reader
	io.Writer
}

func TestAskpassMFA(t *testing.T) {
	h := newHarness(t)
	var out, screen bytes.Buffer
	h.env.Stdout = &out
	h.env.OpenBrowser = func(u string) error { h.opened = u; return nil }
	prompt := "Open https://w.example/mfa/7 (code BCDF-GHJK) or enter TOTP: "
	require.NoError(t, Askpass(h.env, fakeTTY{strings.NewReader("123456\n"), &screen}, prompt))
	assert.Equal(t, "https://w.example/mfa/7", h.opened)
	assert.Equal(t, prompt, screen.String())
	assert.Equal(t, "123456\n", out.String())

	out.Reset()
	screen.Reset()
	require.NoError(t, Askpass(h.env, fakeTTY{strings.NewReader(""), &screen}, "Open https://w.example/mfa/7 (code BCDF-GHJK): "))
	assert.Equal(t, "\n", out.String(), "a passkey prompt is answered empty")
}

// Any other prompt is read from the terminal and goes to ssh only; the browser stays closed.
func TestAskpassOtherPrompt(t *testing.T) {
	h := newHarness(t)
	var out, screen bytes.Buffer
	h.env.Stdout = &out
	h.env.OpenBrowser = func(string) error { t.Error("browser opened"); return nil }
	require.NoError(t, Askpass(h.env, fakeTTY{strings.NewReader("s3cret\n"), &screen}, "alice@gw's password: "))
	assert.Equal(t, "alice@gw's password: ", screen.String())
	assert.Equal(t, "s3cret\n", out.String())
	assert.Error(t, Askpass(h.env, fakeTTY{strings.NewReader(""), &screen}, "Password: "), "no input is no answer")
}

// werbot ssh hands the child ssh the helper through SSH_ASKPASS, and no other Werbot variable.
func TestSSHSetsAskpass(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	h := newHarness(t)
	bin := t.TempDir()
	envFile := filepath.Join(bin, "env")
	stub := "#!/bin/sh\nprintf '%s\\n%s\\n' \"$SSH_ASKPASS\" \"$SSH_ASKPASS_REQUIRE\" > " + envFile + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ssh"), []byte(stub), 0o700)) //nolint:gosec // test stub
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	require.NoError(t, h.run("ssh", "db"))
	self, err := os.Executable()
	require.NoError(t, err)
	got, err := os.ReadFile(envFile) //nolint:gosec // test temp file
	require.NoError(t, err)
	assert.Equal(t, self+"\nforce\n", string(got))
}
