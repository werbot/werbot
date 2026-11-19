package gateway

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// compileSubjects reads who the snapshot lets in: the organization members
// with their teams and the user CA keys whose certificates the SSH front takes.
func compileSubjects(s *gatewayv1.Snapshot) (map[uuid.UUID][]uuid.UUID, [][]byte, error) {
	members, err := compileMembers(s.GetMembers())
	if err != nil {
		return nil, nil, err
	}
	var cas [][]byte
	for _, k := range s.GetUserCaKeys() {
		pub, _, _, _, err := gossh.ParseAuthorizedKey([]byte(k))
		if err != nil {
			return nil, nil, errors.New("snapshot user CA: invalid key")
		}
		cas = append(cas, pub.Marshal())
	}
	return members, cas, nil
}

// IsUserCA reports whether key is a user CA of the snapshot's organization.
// Each organization has its own CA: another organization's certificate fails.
func (p *Policy) IsUserCA(key gossh.PublicKey) bool {
	if p == nil {
		return false
	}
	b := key.Marshal()
	return slices.ContainsFunc(p.userCAs, func(ca []byte) bool { return bytes.Equal(ca, b) })
}

// checkUserCert verifies a user certificate offered for login: signed by the
// organization's CA, login its only principal, inside its validity window at
// now and without critical options (CertChecker supports none). sshca.Parse
// then reads the werbot identity; a certificate without it is refused.
func checkUserCert(p *Policy, login string, cert *gossh.Certificate, now time.Time) (sshca.Identity, error) {
	if cert.CertType != gossh.UserCert {
		return sshca.Identity{}, errors.New("not a user certificate")
	}
	if !p.IsUserCA(cert.SignatureKey) {
		return sshca.Identity{}, errors.New("certificate authority is not the organization's")
	}
	if err := (&gossh.CertChecker{Clock: func() time.Time { return now }}).CheckCert(login, cert); err != nil {
		return sshca.Identity{}, err
	}
	return sshca.Parse(cert)
}

// keyIdentity is the Identity of the key offered for login: "ssh-key" and its
// fingerprint, or "ssh-cert" for a certificate that passes checkUserCert
// against pol. A certificate without a snapshot is ErrNotReady; a refused one
// is PermissionDenied, as the server answers a refused key.
func keyIdentity(pol *Policy, login string, key gossh.PublicKey, now time.Time) (Identity, error) {
	id := Identity{Method: "ssh-key", Value: gossh.FingerprintSHA256(key)}
	cert, ok := key.(*gossh.Certificate)
	if !ok {
		return id, nil
	}
	if pol == nil {
		return id, ErrNotReady
	}
	c, err := checkUserCert(pol, login, cert, now)
	if err != nil {
		return id, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("certificate: %w", err))
	}
	id.Method, id.Cert = "ssh-cert", &c
	return id, nil
}
