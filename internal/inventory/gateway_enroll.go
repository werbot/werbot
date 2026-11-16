package inventory

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/netip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

var (
	// One message for unknown, expired, revoked, exhausted and malformed tokens.
	errInvalidJoinToken = fmt.Errorf("invalid join token: %w", apperr.ErrPermissionDenied)
	errUnknownInstance  = fmt.Errorf("unknown gateway instance: %w", apperr.ErrUnauthenticated)
)

// EnrollRequest is a gateway instance joining a pool.
type EnrollRequest struct {
	JoinToken         string
	CSRDER            []byte // ed25519
	Hostname, Version string
	Connectors        []string
	IP                netip.Addr
}

// Enrollment is what a new instance needs: its certificate, the CA root and the pool host key.
type Enrollment struct {
	GatewayID, InstanceID      uuid.UUID
	CertPEM, CAPEM, HostKeyPEM []byte
}

// issue signs csrDER for the instance and returns the PEM and the parsed certificate.
func (g *Gateways) issue(csrDER []byte, gatewayID, instanceID uuid.UUID) ([]byte, *x509.Certificate, error) {
	certPEM, err := g.ca.IssueClient(csrDER, tlsca.GatewayCN(gatewayID, instanceID), CertTTL)
	if err != nil {
		return nil, nil, err
	}
	blk, _ := pem.Decode(certPEM)
	if blk == nil {
		return nil, nil, errors.New("inventory: issued certificate is not PEM")
	}
	cert, err := x509.ParseCertificate(blk.Bytes)
	return certPEM, cert, err
}

// Enroll spends one use of the join token and registers a new instance of its
// pool. Any failure after the spend rolls it back.
func (g *Gateways) Enroll(ctx context.Context, r EnrollRequest) (Enrollment, error) {
	hash, fingerprint, ok := parseJoinToken(r.JoinToken)
	if !ok {
		return Enrollment{}, errInvalidJoinToken
	}
	if fingerprint != g.ca.Fingerprint() {
		return Enrollment{}, invalid("join token belongs to another control plane")
	}
	var ip *netip.Addr
	ipText := ""
	if r.IP.IsValid() {
		ip, ipText = &r.IP, r.IP.String()
	}
	connectors := r.Connectors
	if connectors == nil {
		connectors = []string{}
	}
	out := Enrollment{CAPEM: g.ca.RootPEM()}
	// The organization is known only after the token is spent.
	err := dbx.System(ctx, g.pool, dbx.ReasonEnroll, func(tx pgx.Tx) error {
		st := newStore(tx)
		tok, err := st.SpendJoinToken(ctx, hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return errInvalidJoinToken
		}
		if err != nil {
			return err
		}
		instanceID, err := uuid.NewV7()
		if err != nil {
			return err
		}
		certPEM, cert, err := g.issue(r.CSRDER, tok.GatewayID, instanceID)
		if err != nil {
			return err
		}
		if err := st.CreateInstance(ctx, dbq.CreateInstanceParams{ID: instanceID, GatewayID: tok.GatewayID, OrgID: tok.OrgID,
			Hostname: r.Hostname, CertSerial: cert.SerialNumber.Text(16), CertExpiresAt: cert.NotAfter,
			Version: r.Version, Connectors: connectors, ClientIp: ip}); err != nil {
			return err
		}
		key, err := st.GetGatewayHostKey(ctx, dbq.GetGatewayHostKeyParams{ID: tok.GatewayID, OrgID: tok.OrgID})
		if err != nil {
			return err
		}
		if out.HostKeyPEM, err = g.box.Open(key.HostKeyCiphertext, key.HostKeyKeyID); err != nil {
			return err
		}
		out.GatewayID, out.InstanceID, out.CertPEM = tok.GatewayID, instanceID, certPEM
		return actor{kind: "gateway", ip: r.IP}.audit(ctx, tx, tok.OrgID, "gateway.instance.enroll", "gateway_instance", instanceID,
			map[string]any{"gateway_id": tok.GatewayID.String(), "instance_id": instanceID.String(), "token_id": tok.ID.String(),
				"hostname": r.Hostname, "client_ip": ipText})
	})
	if err != nil {
		return Enrollment{}, err
	}
	return out, nil
}

// Renew issues a new certificate with the same common name; the old serial stops authenticating.
func (g *Gateways) Renew(ctx context.Context, instanceID uuid.UUID, csrDER []byte) ([]byte, error) {
	var certPEM []byte
	err := dbx.System(ctx, g.pool, dbx.ReasonEnroll, func(tx pgx.Tx) error {
		st := newStore(tx)
		gatewayID, err := st.GetInstanceGatewayID(ctx, instanceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errUnknownInstance
		}
		if err != nil {
			return err
		}
		p, cert, err := g.issue(csrDER, gatewayID, instanceID)
		if err != nil {
			return err
		}
		certPEM = p
		return st.RenewInstanceCert(ctx, dbq.RenewInstanceCertParams{ID: instanceID,
			CertSerial: cert.SerialNumber.Text(16), CertExpiresAt: cert.NotAfter})
	})
	if err != nil {
		return nil, err
	}
	return certPEM, nil
}

// Authenticate checks the ids and serial of a verified client certificate
// against the current instance; any mismatch is ErrUnauthenticated.
func (g *Gateways) Authenticate(ctx context.Context, gatewayID, instanceID uuid.UUID, certSerial string) (Instance, error) {
	var out Instance
	err := dbx.System(ctx, g.pool, dbx.ReasonTunnel, func(tx pgx.Tx) error {
		r, err := newStore(tx).AuthenticateInstance(ctx, dbq.AuthenticateInstanceParams{
			ID: instanceID, GatewayID: gatewayID, CertSerial: certSerial})
		if errors.Is(err, pgx.ErrNoRows) {
			return errUnknownInstance
		}
		if err != nil {
			return err
		}
		out = toInstance(dbq.ListInstancesRow(r))
		return nil
	})
	if err != nil {
		return Instance{}, err
	}
	return out, nil
}

// Heartbeat marks the instance online and stores its version and connectors.
func (g *Gateways) Heartbeat(ctx context.Context, instanceID uuid.UUID, version string, connectors []string) error {
	if connectors == nil {
		connectors = []string{}
	}
	return dbx.System(ctx, g.pool, dbx.ReasonTunnel, func(tx pgx.Tx) error {
		n, err := newStore(tx).HeartbeatInstance(ctx, dbq.HeartbeatInstanceParams{ID: instanceID, Version: version, Connectors: connectors})
		if err == nil && n == 0 {
			return errUnknownInstance
		}
		return err
	})
}
