package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/sshtest"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// apiClients are the Connect clients of the owner "alice" (session cookie).
type apiClients struct {
	keys werbotv1connect.SSHKeyServiceClient
	res  werbotv1connect.ResourceServiceClient
	acc  werbotv1connect.AccessServiceClient
	pol  werbotv1connect.PolicyServiceClient
	gws  werbotv1connect.GatewayServiceClient
}

// stack is werbot server (Run "server"), the pool "gw" with one gateway
// instance (gateway.Serve) and an in-process SSH target.
type stack struct {
	pool      *pgxpool.Pool
	api       apiClients
	key       gossh.Signer // alice's key, added with AddSSHKey
	gatewayID uuid.UUID
	token     string // join token of "gw", max_uses 2: the first instance spent one
	gwSSH     string
	target    *sshtest.Target

	dials, accepted atomic.Int64 // SSH connections of the test / accepted by the gateway listeners

	url    string // http://<HTTPAddr>
	alice  string // alice's user id
	gwData string // DataDir of the first instance
}

// startStack runs the stack on a fresh database with 127.0.0.1 ports only;
// opts go to the SSH target. Everything stops at the test cleanup.
func startStack(t *testing.T, opts ...sshtest.Option) *stack {
	t.Helper()
	s, _ := startStackEnv(t, nil, opts...)
	return s
}

// startStackEnv is startStack with the server variables extra gives for the
// HTTP address (nil: none); it also returns alice's cookie client.
func startStackEnv(t *testing.T, extra func(httpAddr string) map[string]string, opts ...sshtest.Option) (*stack, *http.Client) {
	t.Helper()
	pool, dbURL := dbtest.New(t)
	httpAddr := freeAddr(t)
	vars := map[string]string{
		"WERBOT_DATABASE_URL": dbURL, "WERBOT_HTTP_ADDR": httpAddr, "WERBOT_PUBLIC_URL": "http://" + httpAddr,
		"WERBOT_SECRET_KEY": testSecretKey, "WERBOT_GATEWAY_ADDR": freeAddr(t), "WERBOT_DATA_DIR": t.TempDir(),
		"WERBOT_ENV": "development",
	}
	if extra != nil {
		maps.Copy(vars, extra(httpAddr))
	}
	r := startRun(t, []string{"server"}, env(vars)).healthy(t)
	s := &stack{pool: pool, url: r.api, target: sshtest.StartTarget(t, "target-secret", nil, opts...)}
	ctx := context.Background()

	hc, _ := jarClient(t, http.DefaultTransport)
	auth := werbotv1connect.NewAuthServiceClient(hc, s.url)
	setup := proto.CloneOf(ownerSetup)
	setup.Login, setup.Email = "alice", "alice@example.com"
	_, err := auth.Setup(ctx, connect.NewRequest(setup))
	require.NoError(t, err)
	me, err := auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	s.alice = me.Msg.GetUser().GetId()
	s.api = apiClients{
		keys: werbotv1connect.NewSSHKeyServiceClient(hc, s.url), res: werbotv1connect.NewResourceServiceClient(hc, s.url),
		acc: werbotv1connect.NewAccessServiceClient(hc, s.url), pol: werbotv1connect.NewPolicyServiceClient(hc, s.url),
		gws: werbotv1connect.NewGatewayServiceClient(hc, s.url),
	}

	s.key = s.addKey(t)
	gw, err := s.api.gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "gw"}))
	require.NoError(t, err)
	s.gatewayID = uuid.MustParse(gw.Msg.GetGateway().GetId())
	tok, err := s.api.gws.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{
		GatewayId: s.gatewayID.String(), MaxUses: 2}))
	require.NoError(t, err)
	s.token = tok.Msg.GetToken()
	s.gwSSH, s.gwData, err = s.startGateway(t, s.token)
	require.NoError(t, err)
	return s, hc
}

// addKey registers a new SSH key of alice.
func (s *stack) addKey(t *testing.T) gossh.Signer {
	t.Helper()
	key := newSigner(t)
	_, err := s.api.keys.AddSSHKey(context.Background(), connect.NewRequest(&werbotv1.AddSSHKeyRequest{
		PublicKey: string(gossh.MarshalAuthorizedKey(key.PublicKey()))}))
	require.NoError(t, err)
	return key
}

func newSigner(t *testing.T) gossh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := gossh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return signer
}

// countingListener counts the connections the gateway accepts.
type countingListener struct {
	net.Listener
	n *atomic.Int64
}

func (l countingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		l.n.Add(1)
	}
	return c, err
}

var tunnelUp = regexp.MustCompile(`(?m)^werbot_gateway_tunnel_connected 1$`)

// startGateway runs a gateway instance in a new DataDir enrolling with token
// and waits until its tunnel has a snapshot or Serve returned (its error).
func (s *stack) startGateway(t *testing.T, token string) (sshAddr, dataDir string, err error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	dataDir = t.TempDir()
	err = runUntilUp(t, func(ctx context.Context, reg *metrics.Registry) error {
		cfg := gateway.Config{ServerURL: s.url, JoinToken: token, DataDir: dataDir, Name: "gw-test",
			IdleTimeout: time.Minute, DrainTimeout: time.Second, MaxSessions: 100, Metrics: reg}
		return gateway.Serve(ctx, cfg, countingListener{ln, &s.accepted}, slog.New(slog.DiscardHandler))
	})
	if err != nil {
		return "", dataDir, err
	}
	return ln.Addr().String(), dataDir, nil
}

// runUntilUp runs the gateway instance of run (metrics in reg) until the test
// cleanup and waits until its tunnel has a snapshot or run returned (its error).
func runUntilUp(t *testing.T, run func(ctx context.Context, reg *metrics.Registry) error) error {
	t.Helper()
	reg := metrics.NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	var serveErr error
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		serveErr = run(ctx, reg)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(15 * time.Second):
			t.Error("gateway did not stop within 15s")
		}
	})
	require.Eventually(t, func() bool {
		select {
		case <-stopped:
			return true
		default:
		}
		rec := httptest.NewRecorder()
		reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		return tunnelUp.MatchString(rec.Body.String())
	}, 15*time.Second, 20*time.Millisecond)
	select {
	case <-stopped:
		return serveErr
	default:
		return nil
	}
}

// addResource creates a resource of typ in the pool "gw" with the target
// password and binds it to alice. An ssh resource points at the target.
func (s *stack) addResource(t *testing.T, name, typ string) uuid.UUID {
	t.Helper()
	host, port := hostPort(t, s.target.Addr)
	var cfg proto.Message = &sshv1.Config{Host: host, Port: port, Login: "root", PortForwarding: true}
	if typ == "echo" {
		cfg = &wrapperspb.StringValue{Value: "echo"} // the echo type requires a non-empty value
	}
	return s.createResource(t, s.gatewayID, name, typ, cfg, s.target.Password, true)
}

// dial opens an SSH connection to addr as user with key and returns the host
// key the gateway presented.
func (s *stack) dial(addr, user string, key gossh.Signer) (*gossh.Client, gossh.PublicKey, error) {
	s.dials.Add(1)
	var host gossh.PublicKey
	c, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{User: user, Auth: []gossh.AuthMethod{gossh.PublicKeys(key)},
		HostKeyCallback: func(_ string, _ net.Addr, k gossh.PublicKey) error { host = k; return nil }, Timeout: 10 * time.Second})
	return c, host, err
}

// exec runs cmd as alice without a PTY (execKey).
func (s *stack) exec(t *testing.T, addr, user, cmd string) (out string, code int) {
	t.Helper()
	return s.execKey(t, s.key, addr, user, cmd)
}

// instanceIdentity reads the gateway and instance ids an instance saved in dataDir.
func instanceIdentity(t *testing.T, dataDir string) (gatewayID, instanceID uuid.UUID, tunnelURL string) {
	t.Helper()
	var st struct {
		GatewayID  uuid.UUID `json:"gateway_id"`
		InstanceID uuid.UUID `json:"instance_id"`
		TunnelURL  string    `json:"tunnel_url"`
	}
	b, err := os.ReadFile(filepath.Join(dataDir, "gateway.json")) //nolint:gosec // test temp dir
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &st))
	return st.GatewayID, st.InstanceID, st.TunnelURL
}

// readData reads a file of an instance's DataDir.
func readData(t *testing.T, dataDir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dataDir, name)) //nolint:gosec // test temp dir
	require.NoError(t, err)
	return b
}

// instanceClient is a tunnel client with the certificate of the instance in dataDir.
func instanceClient(t *testing.T, dataDir string) gatewayv1connect.TunnelServiceClient {
	t.Helper()
	cert, err := tls.X509KeyPair(readData(t, dataDir, "gateway.crt"), readData(t, dataDir, "gateway.key"))
	require.NoError(t, err)
	_, _, turl := instanceIdentity(t, dataDir)
	c, _ := tunnelClient(t, turl, readData(t, dataDir, "ca.crt"), cert)
	return c
}

// services are newServices on the stack's database with the server's secret key and CA.
func (s *stack) services(t *testing.T) *services {
	t.Helper()
	o := testServeOpts(t, config.Config{SecretKey: bytes.Repeat([]byte{1}, 32)}, s.pool)
	svc, err := newServices(context.Background(), config.Config{DataDir: t.TempDir()}, s.pool, o.Box, o.CA, o.Signer, metrics.NewRegistry(), slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	return svc
}
