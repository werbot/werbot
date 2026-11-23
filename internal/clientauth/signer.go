package clientauth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// Signer returns the profile key with a user certificate that has at least
// sshca.RenewBefore left, renewing it when needed. When the server cannot
// renew, a still valid certificate is used as is.
func (p *Profile) Signer(ctx context.Context, home string) (ssh.Signer, error) {
	dir := Dir(home, p.Server)
	key, err := loadKey(filepath.Join(dir, "id_ed25519"))
	if err != nil {
		return nil, err
	}
	certPath := filepath.Join(dir, "id_ed25519-cert.pub")
	cert := readCert(certPath, key.PublicKey())
	if cert != nil && time.Until(validBefore(cert)) >= sshca.RenewBefore {
		return ssh.NewCertSigner(cert, key)
	}
	fresh, err := issueCert(ctx, p.HTTPClient(), p.Server, key.PublicKey())
	if err != nil {
		if cert != nil && time.Until(validBefore(cert)) > 0 {
			return ssh.NewCertSigner(cert, key) // server unreachable: keep the valid certificate
		}
		return nil, err
	}
	// valid_before = server now + UserCertTTL, so the difference is the clock offset.
	if skew := time.Until(validBefore(fresh)) - sshca.UserCertTTL; skew.Abs() > sshca.ClockSkew {
		return nil, fmt.Errorf("clock skew %s between this computer and %s; sync the system clock", skew.Abs().Round(time.Minute), p.Server)
	}
	if err := WriteFile(certPath, ssh.MarshalAuthorizedKey(fresh)); err != nil {
		return nil, err
	}
	return ssh.NewCertSigner(fresh, key)
}

func validBefore(c *ssh.Certificate) time.Time {
	return time.Unix(int64(c.ValidBefore), 0) //nolint:gosec // certificate times fit in int64
}

// loadKey reads the profile key or creates an ed25519 one (0600).
func loadKey(path string) (ssh.Signer, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the user's own profile directory
	if err == nil {
		return ssh.ParsePrivateKey(raw)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, err
	}
	if err := WriteFile(path, pem.EncodeToMemory(block)); err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(priv)
}

// readCert returns the saved certificate of pub, or nil when there is none.
func readCert(path string, pub ssh.PublicKey) *ssh.Certificate {
	raw, err := os.ReadFile(path) //nolint:gosec // the user's own profile directory
	if err != nil {
		return nil
	}
	cert, _ := parseCert(raw, pub)
	return cert
}

// parseCert parses an authorized_keys certificate and checks it is for pub.
func parseCert(raw []byte, pub ssh.PublicKey) (*ssh.Certificate, error) {
	k, _, _, _, err := ssh.ParseAuthorizedKey(raw)
	if err != nil {
		return nil, err
	}
	cert, ok := k.(*ssh.Certificate)
	if !ok || !bytes.Equal(cert.Key.Marshal(), pub.Marshal()) {
		return nil, errors.New("certificate is not for the profile key")
	}
	return cert, nil
}

func issueCert(ctx context.Context, hc *http.Client, server string, pub ssh.PublicKey) (*ssh.Certificate, error) {
	res, err := werbotv1connect.NewCertServiceClient(hc, server).IssueUserCert(ctx,
		connect.NewRequest(&werbotv1.IssueUserCertRequest{PublicKey: string(ssh.MarshalAuthorizedKey(pub))}))
	if err != nil {
		return nil, fmt.Errorf("issue a certificate: %w", err)
	}
	cert, err := parseCert([]byte(res.Msg.GetCertificate()), pub)
	if err != nil {
		return nil, fmt.Errorf("issue a certificate: %w", err)
	}
	return cert, nil
}
