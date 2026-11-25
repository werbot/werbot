package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
)

// writeRunDir fills the run directory dir (0700, files 0600): the run key and
// certificate (call a.Signer first), known_hosts of res, an ssh_config with a
// block per ssh resource of res, and bin/ssh that runs ssh with that config.
func writeRunDir(dir string, a *clientauth.AgentProfile, res []*werbotv1.AccessibleResource, sshPath, goos string) error {
	if err := a.WriteKey(dir); err != nil {
		return err
	}
	if err := writeKnownHosts(filepath.Join(dir, "known_hosts"), res); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := clientauth.WriteFile(filepath.Join(dir, "ssh_config"), []byte(runSSHConfig(dir, exe, a.Login, res, goos))); err != nil {
		return err
	}
	return writeSSHWrapper(dir, sshPath, goos)
}

// runSSHConfig has one Host <resource> block for every ssh resource with a
// gateway address; only the run key is offered, never an SSH agent's keys. A
// WebSocket-only pool is reached through "<exe> proxy <ws_url>": the child
// runs without the profile location, so the proxy never looks a name up.
func runSSHConfig(dir, exe, login string, res []*werbotv1.AccessibleResource, goos string) string {
	var b strings.Builder
	for _, r := range res {
		lines, err := hostLines(r, proxyCommand(exe, r.GetWsUrl(), goos))
		if r.GetType() != "ssh" || err != nil {
			continue
		}
		fmt.Fprintf(&b, "Host %s\n%s  User %s+%s\n  IdentityFile %s\n  CertificateFile %s\n  IdentitiesOnly yes\n  IdentityAgent none\n  UserKnownHostsFile %s\n  StrictHostKeyChecking yes\n",
			r.GetName(), lines, login, r.GetName(),
			filepath.Join(dir, "id_ed25519"), filepath.Join(dir, "id_ed25519-cert.pub"), filepath.Join(dir, "known_hosts"))
	}
	return b.String()
}

// wrapperPath is the ssh wrapper of the run directory.
func wrapperPath(dir, goos string) string {
	if goos == "windows" {
		return filepath.Join(dir, "bin", "ssh.cmd")
	}
	return filepath.Join(dir, "bin", "ssh")
}

// writeSSHWrapper writes the executable that runs sshPath with the run ssh_config.
func writeSSHWrapper(dir, sshPath, goos string) error {
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		return err
	}
	cfg := filepath.Join(dir, "ssh_config")
	script := "#!/bin/sh\nexec " + shellQuote(sshPath) + " -F " + shellQuote(cfg) + " \"$@\"\n"
	if goos == "windows" { // cmd expands %NAME% in a script: a literal % is %%
		script = `@"` + strings.ReplaceAll(sshPath, "%", "%%") + `" -F "` + strings.ReplaceAll(cfg, "%", "%%") + `" %*` + "\r\n"
	}
	return os.WriteFile(wrapperPath(dir, goos), []byte(script), 0o700) //nolint:gosec // an executable in the 0700 run directory
}

// shellQuote quotes s as one POSIX shell word (cli may not import gateway).
// ponytail: copy of gateway.ShellQuote. syntax.Quote(LangPOSIX) refuses
// non-printable runes and invalid UTF-8, which this quotes; keep both until
// such paths may fail the wrapper.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
