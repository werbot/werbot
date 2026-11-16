package gateway

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/tlsca"
	"github.com/werbot/werbot/internal/platform/version"
)

const stateFile = "gateway.json"

// state is the persisted instance identity.
type state struct {
	GatewayID  uuid.UUID `json:"gateway_id"`
	InstanceID uuid.UUID `json:"instance_id"`
	TunnelURL  string    `json:"tunnel_url"`
}

// ensureEnrolled returns the persisted identity, enrolling through the join token on first start.
// A saved identity wins: the token is not read, so an exhausted one in env is harmless.
func ensureEnrolled(ctx context.Context, cfg Config, hc *http.Client, connectors []string) (state, *certStore, *x509.CertPool, error) {
	var st state
	var cs *certStore
	var err error
	if Enrolled(cfg.DataDir) {
		st, cs, err = loadIdentity(cfg.DataDir)
	} else {
		st, cs, err = enroll(ctx, cfg, hc, connectors)
	}
	if err != nil {
		return state{}, nil, nil, err
	}
	pool := x509.NewCertPool()
	caPEM, err := os.ReadFile(filepath.Join(cfg.DataDir, caFile))
	if err != nil || !pool.AppendCertsFromPEM(caPEM) {
		return state{}, nil, nil, fmt.Errorf("read %s: missing or not a certificate", caFile)
	}
	if cfg.TunnelURL != "" {
		st.TunnelURL = cfg.TunnelURL
	}
	return st, cs, pool, nil
}

// singleCA parses the first PEM block as the only trusted root and returns the pool plus its canonical PEM.
// Trailing blocks are ignored, never trusted.
func singleCA(caPEM []byte) (*x509.CertPool, []byte, error) {
	blk, _ := pem.Decode(caPEM)
	if blk == nil || blk.Type != "CERTIFICATE" {
		return nil, nil, errors.New("not a PEM certificate")
	}
	ca, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		return nil, nil, errors.New("invalid CA certificate")
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return pool, pem.EncodeToMemory(blk), nil
}

// verifyClientCert checks that certPEM chains to pool for client auth.
func verifyClientCert(certPEM []byte, pool *x509.CertPool) error {
	blk, _ := pem.Decode(certPEM)
	if blk == nil || blk.Type != "CERTIFICATE" {
		return errors.New("enroll: certificate is not PEM")
	}
	leaf, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		return errors.New("enroll: invalid certificate in response")
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return fmt.Errorf("enroll: certificate does not chain to the pinned CA: %w", err)
	}
	return nil
}

func loadIdentity(dir string) (state, *certStore, error) {
	var st state
	b, err := os.ReadFile(filepath.Join(dir, stateFile)) //nolint:gosec // path is DataDir + fixed name
	if err == nil {
		err = json.Unmarshal(b, &st)
	}
	if err != nil {
		return state{}, nil, fmt.Errorf("read %s: %w", stateFile, err)
	}
	cs, err := loadCertStore(dir)
	return st, cs, err
}

func enroll(ctx context.Context, cfg Config, hc *http.Client, connectors []string) (state, *certStore, error) {
	if cfg.ServerURL == "" || cfg.JoinToken == "" {
		return state{}, nil, errors.New("instance is not enrolled: WERBOT_GATEWAY_SERVER_URL and WERBOT_GATEWAY_JOIN_TOKEN are required")
	}
	wantFP, err := ParseJoinToken(cfg.JoinToken)
	if err != nil {
		return state{}, nil, err
	}
	keyPEM, csr, err := newCSR()
	if err != nil {
		return state{}, nil, err
	}
	resp, err := werbotv1connect.NewGatewayEnrollServiceClient(hc, cfg.ServerURL).Enroll(ctx, connect.NewRequest(&werbotv1.EnrollRequest{
		JoinToken: strings.TrimSpace(cfg.JoinToken), CsrDer: csr, Hostname: cfg.Name, Version: version.Version, Connectors: connectors,
	}))
	if err != nil {
		return state{}, nil, fmt.Errorf("enroll: %w", err)
	}
	m := resp.Msg
	gotFP, err := tlsca.FingerprintPEM([]byte(m.GetCaPem()))
	if err != nil {
		return state{}, nil, fmt.Errorf("enroll: %w", err)
	}
	if gotFP != wantFP {
		return state{}, nil, errors.New("enroll: CA fingerprint mismatch: server CA does not match the join token")
	}
	caPool, caPEM, err := singleCA([]byte(m.GetCaPem()))
	if err != nil {
		return state{}, nil, fmt.Errorf("enroll: %w", err)
	}
	if err := verifyClientCert([]byte(m.GetCertPem()), caPool); err != nil {
		return state{}, nil, err
	}
	st := state{TunnelURL: m.GetTunnelUrl()}
	if st.GatewayID, err = uuid.Parse(m.GetGatewayId()); err != nil {
		return state{}, nil, errors.New("enroll: invalid gateway_id in response")
	}
	if st.InstanceID, err = uuid.Parse(m.GetInstanceId()); err != nil {
		return state{}, nil, errors.New("enroll: invalid instance_id in response")
	}
	if _, err := gossh.ParsePrivateKey([]byte(m.GetHostKeyPem())); err != nil {
		return state{}, nil, errors.New("enroll: invalid host key in response")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return state{}, nil, err
	}
	if err := os.Chmod(cfg.DataDir, 0o700); err != nil { //nolint:gosec // directory needs the x bit; fixes pre-existing dirs/volumes
		return state{}, nil, err
	}
	cs := &certStore{dir: cfg.DataDir}
	if err := cs.Replace([]byte(m.GetCertPem()), keyPEM); err != nil {
		return state{}, nil, err
	}
	if err := writeAtomic(filepath.Join(cfg.DataDir, caFile), caPEM, 0o644); err != nil {
		return state{}, nil, err
	}
	if err := writeAtomic(filepath.Join(cfg.DataDir, hostKeyFile), []byte(m.GetHostKeyPem()), 0o600); err != nil {
		return state{}, nil, err
	}
	b, err := json.Marshal(st)
	if err != nil {
		return state{}, nil, err
	}
	// gateway.json last: it marks the identity as complete
	if err := writeAtomic(filepath.Join(cfg.DataDir, stateFile), b, 0o600); err != nil {
		return state{}, nil, err
	}
	return st, cs, nil
}
