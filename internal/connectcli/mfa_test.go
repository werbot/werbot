package connectcli

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mfaURL     = "https://w.example/mfa/7?x=1"
	totpPrompt = "Open " + mfaURL + " (code BCDF-GHJK) or enter TOTP: "
	keyPrompt  = "Open " + mfaURL + " (code BCDF-GHJK): "
)

func TestParseMFAPrompt(t *testing.T) {
	url, code, totp, ok := ParseMFAPrompt(totpPrompt)
	assert.True(t, ok)
	assert.Equal(t, mfaURL, url)
	assert.Equal(t, "BCDF-GHJK", code)
	assert.True(t, totp)

	url, _, totp, ok = ParseMFAPrompt(keyPrompt)
	assert.True(t, ok)
	assert.Equal(t, mfaURL, url)
	assert.False(t, totp)

	for _, s := range []string{
		"Password: ", "Open https://w.example/mfa/7: ", "Open https://w.example/m (code ABCD-EFGH): ", // vowels are not in the alphabet
		"Open file:///etc/passwd (code BCDF-GHJK): ", "Open javascript:alert(1) (code BCDF-GHJK): ",
		"Open ftp://w.example (code BCDF-GHJK): ", totpPrompt + "\n", "x" + keyPrompt,
	} {
		_, _, _, ok = ParseMFAPrompt(s)
		assert.False(t, ok, s)
	}
}

type spyReader struct{ n int }

func (s *spyReader) Read([]byte) (int, error) { s.n++; return 0, io.EOF }

func TestKeyboardInteractiveMFA(t *testing.T) {
	var opened []string
	open := func(u string) error { opened = append(opened, u); return errors.New("no browser") } // an error is ignored
	var out bytes.Buffer

	got, err := KeyboardInteractive(open, strings.NewReader("123456\n"), &out)("", "", []string{totpPrompt}, []bool{false})
	require.NoError(t, err)
	assert.Equal(t, []string{"123456"}, got)
	assert.Equal(t, []string{mfaURL}, opened)
	assert.Contains(t, out.String(), totpPrompt)
	assert.NotContains(t, out.String(), "123456", "a TOTP is never echoed or logged")

	in := &spyReader{}
	out.Reset()
	got, err = KeyboardInteractive(open, in, &out)("", "", []string{keyPrompt}, []bool{false})
	require.NoError(t, err)
	assert.Equal(t, []string{""}, got)
	assert.Zero(t, in.n, "a passkey prompt answers without reading input")
	assert.Contains(t, out.String(), "Opened "+mfaURL+" (code BCDF-GHJK), waiting for confirmation")
}

// A prompt that is not the MFA one never opens the browser and is never answered by itself.
func TestKeyboardInteractiveUnrelatedPrompt(t *testing.T) {
	opened := false
	open := func(string) error { opened = true; return nil }
	var out bytes.Buffer
	got, err := KeyboardInteractive(open, strings.NewReader("hunter2\n"), &out)("", "", []string{"Password: "}, []bool{false})
	require.NoError(t, err)
	assert.Equal(t, []string{"hunter2"}, got)
	assert.Contains(t, out.String(), "Password: ")

	for _, q := range []string{"Password: ", "Open file:///x (code BCDF-GHJK): "} {
		_, err = KeyboardInteractive(open, &spyReader{}, &out)("", "", []string{q}, []bool{false})
		assert.Error(t, err, "no input is no answer: %s", q)
	}
	assert.False(t, opened)
}

func TestConnectMFA(t *testing.T) {
	for _, tc := range []struct{ name, prompt, in, want string }{
		{"totp", totpPrompt, "123456\n", "123456"},
		{"webauthn", keyPrompt, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port := freePortNum(t)
			g := startMFAGateway(t, tcpInfo, tc.prompt, tc.want)
			_, cfg := testSetup(t, tcpInfo, strconv.Itoa(int(port))+":pg")
			cfg.Gateway, cfg.KnownHosts = g.addr, filepath.Join(t.TempDir(), "kh")
			var opened []string
			cfg.OpenBrowser = func(u string) error { opened = append(opened, u); return nil }
			cfg.Stdin = strings.NewReader(tc.in)
			r := startRun(t, cfg)
			r.ready(t, "pg", port, "tcp")
			assert.Equal(t, []string{mfaURL}, opened)
			assert.Equal(t, []string{tc.want}, g.received())
			assert.NotContains(t, r.stderr.String(), "123456")
		})
	}
}
