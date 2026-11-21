// Package sshca signs and parses the short-lived SSH user certificates that werbot issues.
package sshca

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

// Certificate extensions carrying the werbot identity, and certificate timing.
const (
	ExtOrg      = "werbot-org@werbot.com"
	ExtUser     = "werbot-user@werbot.com"
	ExtSession  = "werbot-session@werbot.com"
	ExtAgent    = "werbot-agent@werbot.com"
	ExtRun      = "werbot-run@werbot.com"
	UserCertTTL = time.Hour
	RenewBefore = 10 * time.Minute
	ClockSkew   = time.Minute
)

// Identity is the werbot subject a user certificate is issued to.
type Identity struct {
	OrgID, UserID, CLISessionID uuid.UUID
	Login                       string
	AgentID, RunID              *uuid.UUID
}

// Sign issues a user certificate for pub, valid from now-ClockSkew to now+ttl,
// with the single principal id.Login and the identity in extensions.
func Sign(ca ssh.Signer, pub ssh.PublicKey, id Identity, now time.Time, ttl time.Duration) (*ssh.Certificate, error) {
	var serial [8]byte
	_, _ = rand.Read(serial[:]) // crypto/rand.Read never returns an error
	ext := map[string]string{
		"permit-pty":             "",
		"permit-port-forwarding": "",
		ExtOrg:                   id.OrgID.String(),
		ExtUser:                  id.UserID.String(),
		ExtSession:               id.CLISessionID.String(),
	}
	if id.AgentID != nil {
		ext[ExtAgent] = id.AgentID.String()
	}
	if id.RunID != nil {
		ext[ExtRun] = id.RunID.String()
	}
	cert := &ssh.Certificate{
		Key:             pub,
		Serial:          binary.BigEndian.Uint64(serial[:]),
		CertType:        ssh.UserCert,
		KeyId:           id.UserID.String(),
		ValidPrincipals: []string{id.Login},
		ValidAfter:      uint64(now.Add(-ClockSkew).Unix()), //nolint:gosec // unix time after 1970
		ValidBefore:     uint64(now.Add(ttl).Unix()),        //nolint:gosec // unix time after 1970
		Permissions:     ssh.Permissions{Extensions: ext},
	}
	if err := cert.SignCert(rand.Reader, ca); err != nil {
		return nil, fmt.Errorf("sshca: sign: %w", err)
	}
	return cert, nil
}

// Parse reads the identity from a user certificate. It does not verify the
// signature or validity window; the caller runs ssh.CertChecker first.
func Parse(cert *ssh.Certificate) (Identity, error) {
	if cert.CertType != ssh.UserCert {
		return Identity{}, errors.New("sshca: not a user certificate")
	}
	if len(cert.ValidPrincipals) != 1 || cert.ValidPrincipals[0] == "" {
		return Identity{}, errors.New("sshca: want exactly one principal")
	}
	id := Identity{Login: cert.ValidPrincipals[0]}
	for name, dst := range map[string]*uuid.UUID{ExtOrg: &id.OrgID, ExtUser: &id.UserID, ExtSession: &id.CLISessionID} {
		v, ok := cert.Extensions[name]
		if !ok {
			return Identity{}, fmt.Errorf("sshca: missing extension %s", name)
		}
		u, err := uuid.Parse(v)
		if err != nil {
			return Identity{}, fmt.Errorf("sshca: invalid extension %s", name)
		}
		*dst = u
	}
	for name, dst := range map[string]**uuid.UUID{ExtAgent: &id.AgentID, ExtRun: &id.RunID} {
		v, ok := cert.Extensions[name]
		if !ok {
			continue
		}
		u, err := uuid.Parse(v)
		if err != nil {
			return Identity{}, fmt.Errorf("sshca: invalid extension %s", name)
		}
		*dst = &u
	}
	if cert.KeyId != id.UserID.String() {
		return Identity{}, fmt.Errorf("sshca: key id does not match %s", ExtUser)
	}
	return id, nil
}
