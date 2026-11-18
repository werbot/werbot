//go:build cluster || load

package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/knownhosts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	udpv1 "github.com/werbot/werbot/gen/werbot/connector/udp/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// connectReady bounds the wait for the resources on both instances and for every listen line of werbot connect.
const connectReady = 30 * time.Second

// werbotBin is the host build of werbot (Up), used for werbot connect.
var werbotBin = filepath.Join(standDir, ".bin", "werbot")

// buildWerbot builds werbot for this host into .bin.
func buildWerbot(ctx context.Context) error { return goBuild(ctx, nil, werbotBin, "./cmd/werbot") }

// Connect is a running werbot connect.
type Connect struct {
	// Ports maps a resource to the 127.0.0.1:port werbot connect listens on;
	// with several specs of one resource the last listen line wins.
	Ports map[string]string
}

// StartConnect runs werbot connect as user with alice's key through the pool
// front (nginx :2222) for specs ([<local_port>:]<resource>[:<target_port>]).
// It first waits until every resource of specs answers on gw1 and gw2, then
// until werbot connect printed "<resource> → 127.0.0.1:<port> (tcp|udp)" for
// each spec. Cleanup sends SIGINT and expects the exit within 1 s.
func (s *Stand) StartConnect(t testing.TB, user string, specs ...string) *Connect {
	t.Helper()
	return s.StartConnectAt(t, s.Addr(t, "nginx", 2222), user, specs...)
}

// StartConnectAt is StartConnect through the pool front at pool, an address of
// nginx :2222 other than the published one (the load tests dial the bridge IP).
func (s *Stand) StartConnectAt(t testing.TB, pool, user string, specs ...string) *Connect {
	t.Helper()
	s.waitResources(t, user, specs)
	kh := filepath.Join(t.TempDir(), "known_hosts") // ruling C9; the pool key only: a different key fails
	require.NoError(t, os.WriteFile(kh, []byte(knownhosts.Line([]string{knownhosts.Normalize(pool)}, s.HostKey)+"\n"), 0o600))
	args := append([]string{"connect", "--gateway", pool, "--login", user, "-i", s.KeyFile, "--known-hosts", kh}, specs...)
	cmd := exec.Command(werbotBin, args...) //nolint:gosec // host build of werbot; no context: Cleanup stops it with SIGINT
	var stdout, stderr syncBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Start())
	done := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-done:
			assert.NoError(t, waitErr, "werbot connect exit; stderr:\n%s", stderr.String())
		case <-time.After(time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Errorf("werbot connect did not exit within 1s of SIGINT; stderr:\n%s", stderr.String())
		}
	})

	var exited atomic.Bool
	listening := func() bool {
		select {
		case <-done:
			exited.Store(true)
			return true
		default:
		}
		return len(listenLines(stdout.String())) >= len(specs)
	}
	if !assert.Eventually(t, listening, connectReady, 100*time.Millisecond) || exited.Load() {
		state := "running"
		if exited.Load() {
			state = fmt.Sprintf("exited: %v", waitErr) // set before done closed
		}
		t.Fatalf("werbot connect: %d of %d listen lines (%s)\nstdout:\n%s\nstderr:\n%s",
			len(listenLines(stdout.String())), len(specs), state, stdout.String(), stderr.String())
	}
	c := &Connect{Ports: map[string]string{}}
	for _, f := range listenLines(stdout.String()) {
		c.Ports[f[0]] = f[2]
	}
	return c
}

// listenLines are the "<resource> → 127.0.0.1:<port> (tcp|udp)" lines of out, split into fields.
func listenLines(out string) [][]string {
	var lines [][]string
	for line := range strings.Lines(out) {
		if f := strings.Fields(line); len(f) == 4 && f[1] == "→" && (f[3] == "(tcp)" || f[3] == "(udp)") {
			lines = append(lines, f)
		}
	}
	return lines
}

// waitResources waits until user+<resource> of every spec gets a positive
// resource@werbot.com answer from gw1 and gw2: the bindings reached both instances.
func (s *Stand) waitResources(t testing.TB, user string, specs []string) {
	t.Helper()
	var names []string
	for _, sp := range specs {
		for part := range strings.SplitSeq(sp, ":") {
			if _, err := strconv.Atoi(part); err != nil && !slices.Contains(names, part) {
				names = append(names, part) // the only non-numeric part is the resource
			}
		}
	}
	addrs := []string{s.Addr(t, "nginx", 2201), s.Addr(t, "nginx", 2202)}
	var last atomic.Value // Eventually runs the condition in its own goroutine
	answered := func() bool {
		for _, name := range names {
			for _, addr := range addrs {
				if err := s.resourceInfo(addr, user+"+"+name); err != nil {
					last.Store(err.Error())
					return false
				}
			}
		}
		return true
	}
	if !assert.Eventually(t, answered, connectReady, 500*time.Millisecond) {
		t.Fatalf("resources %v not ready on gw1 and gw2: %v", names, last.Load())
	}
}

// resourceInfo asks the gateway at addr for the resource of login, as werbot connect does.
func (s *Stand) resourceInfo(addr, login string) error {
	c, err := s.dial(addr, login)
	if err != nil {
		return err
	}
	defer c.Close()
	ok, payload, err := c.SendRequest("resource@werbot.com", true, nil)
	switch {
	case err != nil:
		return fmt.Errorf("%s via %s: %w", login, addr, err)
	case !ok:
		return fmt.Errorf("%s via %s: refused %q", login, addr, payload)
	}
	return nil
}

// AddPortResource creates a tcp or udp resource on pool-a from cfg, binds
// alice to it and deletes it in Cleanup.
func (s *Stand) AddPortResource(t testing.TB, name string, cfg proto.Message) uuid.UUID {
	t.Helper()
	typ := "tcp"
	if _, ok := cfg.(*udpv1.Config); ok {
		typ = "udp"
	}
	a, err := anypb.New(cfg)
	require.NoError(t, err)
	res, err := s.Edge.Resources.CreateResource(t.Context(), connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: name, Type: typ, GatewayId: s.PoolID.String(), Config: a,
	}))
	require.NoError(t, err)
	id := res.Msg.GetResource().GetId()
	t.Cleanup(func() {
		_, err := s.Edge.Resources.DeleteResource(context.Background(), connect.NewRequest(&werbotv1.DeleteResourceRequest{Id: id}))
		assert.NoError(t, err)
	})
	_, err = s.Edge.Access.CreateBinding(t.Context(), connect.NewRequest(&werbotv1.CreateBindingRequest{
		SubjectType: "user", SubjectId: s.AliceID.String(), ResourceId: id,
	}))
	require.NoError(t, err)
	return uuid.MustParse(id)
}
