//go:build load

// Package load holds the manual load tests L1–L4 (10 §8.3, budgets §10.1) on
// the cluster stand of test/cluster. Run them with `make load` on Linux: RSS
// and listeners come from /proc. With -short only the helper tests run, on any OS.
package load

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/test/cluster"
)

var (
	sessions = flag.Int("load.sessions", 5000, "SSH sessions in total (L1, L3; L4: split evenly across gw1 and gw2)")
	duration = flag.Duration("load.duration", 10*time.Minute, "L1/L3 session length")
	rate     = flag.Int("load.rate", 100, "new SSH sessions per second")
	idle     = flag.Duration("load.idle", 5*time.Minute, "L4 hold time")
	tcpConns = flag.Int("load.tcp", 500, "L1 mix: TCP connections through werbot connect (stage 6b)")
	udpBinds = flag.Int("load.udp", 100, "L1 mix: UDP bindings through werbot connect (stage 6b)")
)

// st is the stand of this process; nil with -short. `make load` runs one load test per process.
var st *cluster.Stand

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}
	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "load tests need Linux (/proc)")
		os.Exit(1)
	}
	// a client and a gateway->target connection per session on this host, both ends counted;
	// the L1 mix adds a client and an echo end per TCP connection and a socket per UDP binding
	need := 4*max(*sessions, 0) + 2*max(*tcpConns, 0) + max(*udpBinds, 0) + 1024
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil || lim.Cur < uint64(need) {
		fmt.Fprintf(os.Stderr, "open files limit %d is below %d: raise it with ulimit -n 65536 (%v)\n", lim.Cur, need, err)
		os.Exit(1)
	}
	ctx := context.Background()
	var err error
	// idle L4 sessions outlive the default 5m gateway idle timeout
	if st, err = cluster.Up(ctx, cluster.Options{Env: map[string]string{"GW_IDLE_TIMEOUT": "1h"}}); err != nil {
		fmt.Fprintf(os.Stderr, "cluster stand: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	if code != 0 {
		_ = os.RemoveAll(filepath.Dir(st.KeyFile)) // alice's key; make load removes the stand next
		os.Exit(code)
	}
	if err := st.Down(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "cluster stand: %v\n", err)
		os.Exit(1)
	}
}

// needStand skips a load test without the stand (-short).
func needStand(t *testing.T) {
	t.Helper()
	if st == nil {
		t.Skip("load test: needs Linux and the stand, run it with make load")
	}
}

var loadTarget struct {
	sync.Once
	id        uuid.UUID
	setOutput func(bytesPerSec int)
}

// useTarget starts the SSH target once per process and creates the resource
// "load-target" on pool-a (password credential, alice bound); it returns the
// resource id and the output control of the target.
func useTarget(t *testing.T) (uuid.UUID, func(bytesPerSec int)) {
	t.Helper()
	loadTarget.Do(func() {
		port, setOutput := startTarget(t)
		ctx := context.Background()
		cfg, err := anypb.New(&sshv1.Config{Host: "host.docker.internal", Port: uint32(port), Login: "load"}) //nolint:gosec // a TCP port fits uint32
		require.NoError(t, err)
		res, err := st.Edge.Resources.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
			Name: "load-target", Type: "ssh", GatewayId: st.PoolID.String(), Config: cfg,
		}))
		require.NoError(t, err)
		id := res.Msg.GetResource().GetId()
		_, err = st.Edge.Resources.SetCredential(ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{
			ResourceId: id, Kind: "password", Value: []byte(targetPassword),
		}))
		require.NoError(t, err)
		_, err = st.Edge.Access.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
			SubjectType: "user", SubjectId: st.AliceID.String(), ResourceId: id,
		}))
		require.NoError(t, err)
		loadTarget.id, loadTarget.setOutput = uuid.MustParse(id), setOutput
	})
	require.NotNil(t, loadTarget.setOutput, "load target setup failed in an earlier test")
	return loadTarget.id, loadTarget.setOutput
}
