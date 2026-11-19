package cli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// harness is a fake Werbot API plus a profile of alice with a valid token and certificate.
type harness struct {
	werbotv1connect.UnimplementedAuthServiceHandler
	werbotv1connect.UnimplementedResourceServiceHandler
	werbotv1connect.UnimplementedCertServiceHandler

	env           Env
	home, dir     string
	out, errOut   *bytes.Buffer
	srv           *httptest.Server
	vars          map[string]string
	resources     []*werbotv1.AccessibleResource
	hostKey       ssh.PublicKey
	ca, key       ssh.Signer // the test CA and the profile key
	identity      sshca.Identity
	unauth        bool   // the API answers Unauthenticated
	certFail      bool   // IssueUserCert answers Unavailable
	issued        int    // IssueUserCert calls
	opened        string // the URL given to OpenBrowser
	cancelRunning context.CancelFunc
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{home: t.TempDir(), out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(h))
	mux.Handle(werbotv1connect.NewResourceServiceHandler(h))
	mux.Handle(werbotv1connect.NewCertServiceHandler(h))
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, _ *http.Request) { // every refresh is refused
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	})
	h.srv = httptest.NewServer(mux)
	t.Cleanup(h.srv.Close)

	h.hostKey = newPub(t)
	h.resources = []*werbotv1.AccessibleResource{{
		Id: uuid.NewString(), Name: "db", Type: "ssh", Kinds: []string{"terminal", "exec", "stream"},
		SshAddress: "gw.example.com:2222", HostKey: string(ssh.MarshalAuthorizedKey(h.hostKey)),
	}}
	h.vars = map[string]string{"WERBOT_HOME": h.home}
	h.env = Env{
		Getenv: func(k string) string { return h.vars[k] },
		Stdout: h.out, Stderr: h.errOut, Stdin: bytes.NewReader(nil),
		OpenBrowser: func(u string) error { h.opened = u; h.cancelRunning(); return nil },
	}

	p := &clientauth.Profile{
		Server: h.srv.URL, Login: "alice", UserID: uuid.New(), OrgID: uuid.New(),
		AccessToken: "access", RefreshToken: "refresh", AccessExpiry: time.Now().Add(10 * time.Minute),
	}
	require.NoError(t, p.Save(h.home))
	h.dir = clientauth.Dir(h.home, p.Server)
	h.plantKeyAndCert(t, p)
	return h
}

// plantKeyAndCert writes the profile key and a fresh certificate of a test CA.
func (h *harness) plantKeyAndCert(t *testing.T, p *clientauth.Profile) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(key, "")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(h.dir, "id_ed25519"), pem.EncodeToMemory(block), 0o600))
	_, caKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	h.ca, err = ssh.NewSignerFromKey(caKey)
	require.NoError(t, err)
	h.key, err = ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	h.identity = sshca.Identity{OrgID: p.OrgID, UserID: p.UserID, CLISessionID: uuid.New(), Login: p.Login}
	h.plantCert(t, sshca.UserCertTTL)
}

// plantCert writes a certificate of the profile key with the given validity left.
func (h *harness) plantCert(t *testing.T, left time.Duration) *ssh.Certificate {
	t.Helper()
	cert, err := sshca.Sign(h.ca, h.key.PublicKey(), h.identity, time.Now().Add(left-sshca.UserCertTTL), sshca.UserCertTTL)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(h.dir, "id_ed25519-cert.pub"), ssh.MarshalAuthorizedKey(cert), 0o600))
	return cert
}

func newPub(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	k, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return k
}

func (h *harness) GetSession(context.Context, *connect.Request[werbotv1.GetSessionRequest]) (*connect.Response[werbotv1.GetSessionResponse], error) {
	if h.unauth {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	return connect.NewResponse(&werbotv1.GetSessionResponse{
		User: &werbotv1.User{Login: "alice"}, Organization: &werbotv1.Organization{Name: "Acme"},
	}), nil
}

func (h *harness) ListAccessibleResources(context.Context, *connect.Request[werbotv1.ListAccessibleResourcesRequest]) (*connect.Response[werbotv1.ListAccessibleResourcesResponse], error) {
	if h.unauth {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	return connect.NewResponse(&werbotv1.ListAccessibleResourcesResponse{Resources: h.resources}), nil
}

func (h *harness) IssueUserCert(context.Context, *connect.Request[werbotv1.IssueUserCertRequest]) (*connect.Response[werbotv1.IssueUserCertResponse], error) {
	h.issued++
	if h.certFail {
		return nil, connect.NewError(connect.CodeUnavailable, nil)
	}
	cert, err := sshca.Sign(h.ca, h.key.PublicKey(), h.identity, time.Now(), sshca.UserCertTTL)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.IssueUserCertResponse{Certificate: string(ssh.MarshalAuthorizedKey(cert))}), nil
}

// logout forgets the current server, as before the first login.
func (h *harness) logout(t *testing.T) {
	t.Helper()
	require.NoError(t, os.Remove(filepath.Join(h.home, "current")))
}

// run executes the client commands under a werbot root with args.
func (h *harness) run(args ...string) error {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancelRunning = cancel
	defer cancel()
	root := &cobra.Command{Use: "werbot", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(h.out)
	root.SetErr(h.errOut)
	root.AddCommand(Commands(h.env)...)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func removeCert(h *harness) error { return os.Remove(filepath.Join(h.dir, "id_ed25519-cert.pub")) }
