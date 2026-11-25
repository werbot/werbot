package cli

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/connectcli"
)

// gatewayAddr splits the ssh_address of r into host and port.
func gatewayAddr(r *werbotv1.AccessibleResource) (host, port string, err error) {
	addr := r.GetSshAddress()
	if addr == "" {
		return "", "", fmt.Errorf("gateway address for %s is not set; ask an admin to set it on the Gateways page", r.GetName())
	}
	return net.SplitHostPort(addr) // the server stores host:port only
}

// serverHost is the host name of the server URL, without the port.
func serverHost(server string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}

// find returns the accessible resource called name.
func find(rs []*werbotv1.AccessibleResource, name string) (*werbotv1.AccessibleResource, error) {
	for _, r := range rs {
		if r.GetName() == name {
			return r, nil
		}
	}
	return nil, fmt.Errorf("resource %s not found or not accessible", name)
}

// sshArgs is the ssh argv for resource r with the profile key, certificate and
// known_hosts in dir; a WebSocket-only pool is reached through exe proxy.
func sshArgs(dir, exe string, r *werbotv1.AccessibleResource, login string, command []string) ([]string, error) {
	args := []string{
		"-i", filepath.Join(dir, "id_ed25519"),
		"-o", "CertificateFile=" + filepath.Join(dir, "id_ed25519-cert.pub"),
		"-o", "UserKnownHostsFile=" + filepath.Join(dir, "known_hosts"),
		"-o", "StrictHostKeyChecking=yes",
		"-o", "IdentitiesOnly=yes",
	}
	user := login + "+" + r.GetName() + "@"
	if wsOnly(r) {
		args = append(args, "-o", "ProxyCommand="+proxyCommand(exe, r.GetName(), runtime.GOOS), "-o", "HostKeyAlias="+hostKeyAlias(r), user+r.GetName())
		return append(args, command...), nil
	}
	host, port, err := gatewayAddr(r)
	if err != nil {
		return nil, err
	}
	args = append(args, "-p", port, user+host)
	return append(args, command...), nil
}

// hostLines are the address lines of an OpenSSH Host block for r: the
// gateway host and port, or for a WebSocket-only pool the resource name, the
// host key alias and the ProxyCommand proxy.
func hostLines(r *werbotv1.AccessibleResource, proxy string) (string, error) {
	if wsOnly(r) {
		return fmt.Sprintf("  HostName %s\n  HostKeyAlias %s\n  ProxyCommand %s\n", r.GetName(), hostKeyAlias(r), proxy), nil
	}
	host, port, err := gatewayAddr(r)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("  HostName %s\n  Port %s\n", host, port), nil
}

// hostKeyAlias is the known_hosts name of the gateway host key of r.
func hostKeyAlias(r *werbotv1.AccessibleResource) string {
	return knownhosts.Normalize(connectcli.HostKeyAddr(endpoint(r)))
}

// sshConfig is an OpenSSH config block for every resource reachable with ssh.
func sshConfig(dir, serverHost, login string, rs []*werbotv1.AccessibleResource) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# werbot ssh-config for %s; the certificate is renewed by werbot ssh, ssh-config, connect and status\n", serverHost)
	for _, r := range rs {
		if !slices.ContainsFunc(r.GetKinds(), func(k string) bool { return k == "terminal" || k == "exec" || k == "stream" }) {
			continue
		}
		lines, err := hostLines(r, "werbot proxy %h")
		if err != nil {
			fmt.Fprintf(&b, "# %s: gateway address is not set\n", r.GetName())
			continue
		}
		fmt.Fprintf(&b, "Host %s.%s\n%s  User %s+%s\n  IdentityFile %s\n  CertificateFile %s\n  UserKnownHostsFile %s\n  IdentitiesOnly yes\n",
			r.GetName(), serverHost, lines, login, r.GetName(),
			filepath.Join(dir, "id_ed25519"), filepath.Join(dir, "id_ed25519-cert.pub"), filepath.Join(dir, "known_hosts"))
	}
	return b.String()
}

// writeKnownHosts rewrites path with the host key of every gateway in rs, once
// each, under the address the client checks it for (connectcli.HostKeyAddr).
func writeKnownHosts(path string, rs []*werbotv1.AccessibleResource) error {
	var lines []string
	for _, r := range rs {
		if connectcli.HostKeyAddr(endpoint(r)) == "" || r.GetHostKey() == "" {
			continue
		}
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.GetHostKey()))
		if err != nil {
			return fmt.Errorf("host key of %s: %w", r.GetName(), err)
		}
		line := knownhosts.Line([]string{hostKeyAlias(r)}, key)
		if !slices.Contains(lines, line) {
			lines = append(lines, line)
		}
	}
	return clientauth.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"))
}

func sshCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use: "ssh <resource> [-- command...]", Short: "Open an ssh session to a resource with the profile certificate", Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			rs, err := listResources(cmd.Context(), p)
			if err != nil {
				return err
			}
			r, err := find(rs, args[0])
			if err != nil {
				return err
			}
			if !r.GetAllowed() {
				return fmt.Errorf("resource %s is not accessible; run werbot request %s", r.GetName(), r.GetName())
			}
			self, err := os.Executable()
			if err != nil {
				return err
			}
			dir := clientauth.Dir(home, p.Server)
			argv, err := sshArgs(dir, self, r, p.Login, args[1:])
			if err != nil {
				return err
			}
			if _, err := p.Signer(cmd.Context(), home); err != nil {
				return err
			}
			if err := writeKnownHosts(filepath.Join(dir, "known_hosts"), rs); err != nil {
				return err
			}
			c := exec.CommandContext(cmd.Context(), "ssh", argv...) //nolint:gosec // fixed binary, every value is one argv entry
			c.Stdin, c.Stdout, c.Stderr = env.Stdin, env.Stdout, env.Stderr
			// the MFA prompt of the gateway is answered by this binary (see Askpass); only ssh gets the variables
			c.Env = append(os.Environ(), "SSH_ASKPASS="+self, "SSH_ASKPASS_REQUIRE=force")
			return c.Run() // an *exec.ExitError carries the ssh exit code to the process
		},
	}
}

func sshConfigCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use: "ssh-config", Short: "Print an OpenSSH config for the resources you can open", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			rs, err := accessible(cmd.Context(), p)
			if err != nil {
				return err
			}
			if _, err := p.Signer(cmd.Context(), home); err != nil {
				return err
			}
			dir := clientauth.Dir(home, p.Server)
			if err := writeKnownHosts(filepath.Join(dir, "known_hosts"), rs); err != nil {
				return err
			}
			host, err := serverHost(p.Server)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(env.Stdout, sshConfig(dir, host, p.Login, rs))
			return err
		},
	}
}
