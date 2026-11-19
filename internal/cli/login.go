package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
)

func loginCommand(env Env) *cobra.Command {
	var server string
	var device bool
	cmd := &cobra.Command{
		Use: "login [--server URL] [--device]", Short: "Log in to a Werbot server", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := clientauth.Home(env.Getenv)
			if err != nil {
				return err
			}
			if server == "" {
				server = env.Getenv("WERBOT_SERVER")
			}
			if server == "" {
				if cur, err := clientauth.Current(home, env.Getenv); err == nil {
					server = cur.Server
				}
			}
			if server == "" {
				return errors.New("--server is required")
			}
			opts := clientauth.LoginOptions{Out: env.Stdout, OpenBrowser: env.OpenBrowser, Device: device}
			p, err := clientauth.Login(cmd.Context(), home, server, opts)
			if err != nil {
				return err
			}
			signer, err := p.Signer(cmd.Context(), home)
			if err != nil {
				return err
			}
			cert, _ := signer.PublicKey().(*ssh.Certificate)
			_, err = fmt.Fprintf(env.Stdout, "Logged in to %s as %s; certificate valid until %s\n",
				p.Server, p.Login, certExpiry(cert).Format(time.RFC3339))
			return err
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "Werbot server URL (default WERBOT_SERVER, else the last login)")
	cmd.Flags().BoolVar(&device, "device", false, "log in with a code entered in a browser on another device")
	return cmd
}

func logoutCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use: "logout", Short: "Revoke the session and remove the profile", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			if err := p.Logout(cmd.Context(), home); err != nil {
				return err
			}
			_, err = fmt.Fprintf(env.Stdout, "Logged out of %s\n", p.Server)
			return err
		},
	}
}

func statusCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Show the logged-in server, user, organization and certificate", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			sess, err := werbotv1connect.NewAuthServiceClient(p.HTTPClient(), p.Server).
				GetSession(cmd.Context(), connect.NewRequest(&werbotv1.GetSessionRequest{}))
			if err != nil {
				return err
			}
			_, renewErr := p.Signer(cmd.Context(), home) // renews a certificate close to expiry
			_, err = fmt.Fprintf(env.Stdout, "Server: %s\nUser: %s\nOrganization: %s\nToken expires: %s\nCertificate: %s\n",
				p.Server, sess.Msg.GetUser().GetLogin(), sess.Msg.GetOrganization().GetName(),
				p.AccessExpiry.Format(time.RFC3339), certStatus(filepath.Join(clientauth.Dir(home, p.Server), "id_ed25519-cert.pub")))
			return errors.Join(renewErr, err)
		},
	}
}

// certExpiry is the end of the certificate validity.
func certExpiry(c *ssh.Certificate) time.Time {
	if c == nil {
		return time.Time{}
	}
	return time.Unix(int64(c.ValidBefore), 0).UTC() //nolint:gosec // certificate times fit in int64
}

// certStatus describes the saved certificate: "valid until <time>", "expired <time>" or "none".
func certStatus(path string) string {
	raw, err := os.ReadFile(path) //nolint:gosec // the user's own profile directory
	if err != nil {
		return "none"
	}
	k, _, _, _, err := ssh.ParseAuthorizedKey(raw)
	cert, ok := k.(*ssh.Certificate)
	if err != nil || !ok {
		return "none"
	}
	until := certExpiry(cert)
	if time.Now().After(until) {
		return "expired " + until.Format(time.RFC3339)
	}
	return "valid until " + until.Format(time.RFC3339)
}
