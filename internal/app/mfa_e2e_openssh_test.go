//go:build unix

package app

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/cli"
)

// init makes the test binary the SSH_ASKPASS helper of
// TestE2EOpenSSHClientMFA: ssh runs it with the prompt, as it runs werbot
// for werbot ssh, and Run answers through the askpass branch.
func init() {
	self, err := os.Executable()
	if err != nil || !cli.IsAskpass(os.Args[1:], os.Getenv, self) {
		return
	}
	if err := Run(context.Background(), os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "askpass:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var opensshVersion = regexp.MustCompile(`OpenSSH_(\d+)\.(\d+)`)

// systemSSH is the ssh binary, skipping the test without OpenSSH 8.4 or
// newer (SSH_ASKPASS_REQUIRE).
func systemSSH(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("ssh is not in PATH")
	}
	out, _ := exec.Command(bin, "-V").CombinedOutput() //nolint:gosec,noctx // the ssh binary from PATH
	m := opensshVersion.FindSubmatch(out)
	if m == nil {
		t.Skipf("not OpenSSH: %q", out)
	}
	major, _ := strconv.Atoi(string(m[1]))
	minor, _ := strconv.Atoi(string(m[2]))
	if major < 8 || major == 8 && minor < 4 {
		t.Skipf("OpenSSH %s.%s is older than 8.4", m[1], m[2])
	}
	return bin
}

// fakeBrowser puts open and xdg-open first in PATH; they record each URL
// in the returned file instead of starting a browser.
func fakeBrowser(t *testing.T) (pathEnv, opened string) {
	t.Helper()
	dir := t.TempDir()
	opened = filepath.Join(dir, "opened")
	script := "#!/bin/sh\necho \"$1\" >> " + opened + "\n"
	for _, name := range []string{"open", "xdg-open"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700)) //nolint:gosec // test helper script
	}
	return dir + string(os.PathListSeparator) + os.Getenv("PATH"), opened
}

// OpenSSH with SSH_ASKPASS_REQUIRE=force asks werbot (here the test binary)
// for the gateway's MFA prompt: it opens the confirmation page and reads a
// TOTP code from its input when there is no terminal.
func TestE2EOpenSSHClientMFA(t *testing.T) {
	sshBin := systemSSH(t)
	m := mfaSSH(t)
	code := totpAt(t, enrollTOTP(t, m.web), 0)
	keyFile := m.addKeyFile(t)
	self, err := os.Executable()
	require.NoError(t, err)
	pathEnv, opened := fakeBrowser(t)
	host, port, err := net.SplitHostPort(m.gwSSH)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, sshBin, "-F", "/dev/null", "-i", keyFile, "-p", port, //nolint:gosec // test binary, temp files and loopback gateway
		"-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		"-o", "PreferredAuthentications=publickey,keyboard-interactive", "-o", "LogLevel=ERROR",
		"alice+srv@"+host, "echo", "hi")
	cmd.Env = []string{"PATH=" + pathEnv, "HOME=" + t.TempDir(), "SSH_ASKPASS=" + self, "SSH_ASKPASS_REQUIRE=force"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // no controlling terminal: the helper reads its input
	cmd.Stdin = strings.NewReader(code + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err, "stderr: %s", stderr.String())
	assert.Equal(t, "hi\n", string(out))

	urls, err := os.ReadFile(opened) //nolint:gosec // test temp file
	require.NoError(t, err, "the helper opened no page")
	assert.Regexp(t, `^`+regexp.QuoteMeta(m.webURL)+`/mfa/[0-9a-f-]{36}\n$`, string(urls))
}
