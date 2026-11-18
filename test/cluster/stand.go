//go:build cluster || load

// Package cluster is the multi-node docker stand of stage 7a and its harness:
// two server replicas, two web replicas and a two-instance gateway pool behind
// nginx (docker-compose.yaml). Scenarios (tag cluster) and load tests (tag
// load) share it.
package cluster

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// Spec values (global constraints) and derived ones; change them only together with the spec.
const (
	BindingPropagation = 2 * time.Second  // 10 §8.3 #2
	ReconnectMax       = 5 * time.Second  // 10 §9.1
	LostMax            = 3 * time.Minute  // 10 §8.3 #5
	S3Outage           = time.Minute      // 10 §8.3 #12
	NotReadyMax        = 15 * time.Second // unary call deadline 10 s + 5 s
	PGRecoverMax       = 30 * time.Second // /readyz of both replicas after a Postgres restart
	ListenRecoverMax   = 45 * time.Second // pgnotify backoff <= 30 s + 15 s
	UploadDrain        = 90 * time.Second // spool backoff <= 30 s + FlushInterval 10 s + margin
	RestoreMax         = 2 * time.Minute
	PoolMaxConns       = 20 // pool_max_conns in WERBOT_DATABASE_URL of the compose file
)

// Fixed addresses of the public network (docker-compose.yaml, nginx.conf).
const (
	IPNginx   = "10.231.0.10"
	IPServer1 = "10.231.0.11"
	IPServer2 = "10.231.0.12"
	IPGW1     = "10.231.0.21"
	IPGW2     = "10.231.0.22"
	IPWeb1    = "10.231.0.31"
	IPWeb2    = "10.231.0.32"
)

// UIHost is the host of WERBOT_WEB_URL; the harness HTTP client resolves *.werbot.test to 127.0.0.1.
const UIHost = "ui.werbot.test"

// Options are compose interpolation variables (GW_IDLE_TIMEOUT, GW2_MAX_SESSIONS); Up adds JOIN_TOKEN.
type Options struct{ Env map[string]string }

// Stand is a running stand with alice signed in and the pool-a gateways enrolled.
type Stand struct {
	Edge, A, B API // nginx :80 (API), nginx :8081 (server1), nginx :8082 (server2), with alice's cookie
	UI         API // http://ui.werbot.test:<nginx 80> -> web1/web2 -> server, with alice's cookie
	DB         *pgxpool.Pool
	Key        gossh.Signer // alice's ed25519 key; KeyFile holds it in OpenSSH PEM, 0600
	KeyFile    string
	HostKey    gossh.PublicKey // pool host key (first handshake with :2201)

	OrgID, AliceID, PoolID uuid.UUID
	Resources              map[string]uuid.UUID // "target" (bound to alice), "target2" (not bound), "geo" (bound, rule allow NL)
	StartedAt              map[string]time.Time // server1, server2 container start at Up (both start at once on an empty DB)

	project string
	env     map[string]string // one interpolation environment for every compose call
	tmp     string            // KeyFile directory
}

// Up replaces any previous stand with a fresh one. On error the stand is left for inspection.
func Up(ctx context.Context, opts Options) (_ *Stand, err error) {
	s := &Stand{project: Project(), env: maps.Clone(opts.Env), Resources: map[string]uuid.UUID{}, StartedAt: map[string]time.Time{}}
	if s.env == nil {
		s.env = map[string]string{}
	}
	defer func() {
		if err != nil && s.tmp != "" {
			_ = os.RemoveAll(s.tmp) // alice's key; the stand itself stays for inspection
		}
	}()
	if _, err := s.Compose(ctx, "down", "-v", "--remove-orphans"); err != nil {
		return nil, err
	}
	if err := buildEchoTarget(ctx); err != nil {
		return nil, err
	}
	if _, err := s.Compose(ctx, "up", "-d", "--wait", "postgres", "seaweedfs"); err != nil {
		return nil, err
	}
	// both server replicas start at once on the empty database (scenario 6)
	if _, err := s.Compose(ctx, "up", "-d", "server1", "server2", "web1", "web2", "nginx", "target", "echo", "dns"); err != nil {
		return nil, err
	}
	if err := poll(ctx, RestoreMax, func(ctx context.Context) error { return s.ready(ctx, "server1", "server2", "web1", "web2") }); err != nil {
		return nil, err
	}
	for _, svc := range []string{"server1", "server2"} {
		c, err := s.inspect(ctx, svc)
		if err != nil {
			return nil, err
		}
		s.StartedAt[svc] = c.StartedAt
	}
	if err := s.connectDB(ctx); err != nil {
		return nil, err
	}
	if err := s.signIn(ctx); err != nil {
		return nil, err
	}
	if err := s.addKey(ctx); err != nil {
		return nil, err
	}
	if err := s.enrollPool(ctx); err != nil {
		return nil, err
	}
	if err := s.addResources(ctx); err != nil {
		return nil, err
	}
	if err := s.waitSSH(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Down removes the stand: containers, networks and volumes.
func (s *Stand) Down(ctx context.Context) error {
	if s.DB != nil {
		s.DB.Close()
	}
	_, err := s.Compose(ctx, "down", "-v", "--remove-orphans")
	return errors.Join(err, os.RemoveAll(s.tmp))
}

// Healthy brings the stand back to its canonical state: every container up with
// the Up environment, all replicas ready, both instances online and one tunnel
// stream per server replica (gw1 -> server1, gw2 -> server2).
func (s *Stand) Healthy(t testing.TB) {
	t.Helper()
	ctx := context.Background() // not t.Context(): Healthy also runs from t.Cleanup, after that context ends
	_, err := s.Compose(ctx, "up", "-d")
	require.NoError(t, err)
	if err = s.waitHealthy(ctx); err == nil {
		return
	}
	t.Logf("Healthy restarts gw1 gw2 once: %v", err) // the error names the trigger: readiness/online or placement
	_, err = s.Compose(ctx, "restart", "gw1", "gw2")
	require.NoError(t, err)
	if err = s.waitHealthy(ctx); err != nil {
		ps, _ := s.Compose(ctx, "ps", "-a")
		t.Fatalf("stand not healthy: %v\n%s", err, ps)
	}
}

// waitHealthy waits RestoreMax for readiness and both instances online, then
// 2*ReconnectMax for the canonical stream placement; the error carries the last state.
func (s *Stand) waitHealthy(ctx context.Context) error {
	err := poll(ctx, RestoreMax, func(ctx context.Context) error {
		if err := s.ready(ctx, "server1", "server2", "web1", "web2", "gw1", "gw2"); err != nil {
			return err
		}
		return s.poolOnline(ctx)
	})
	if err != nil {
		return fmt.Errorf("readiness/online: %w", err)
	}
	err = poll(ctx, 2*ReconnectMax, func(ctx context.Context) error {
		var got [2]int64
		for i, svc := range []string{"server1", "server2"} {
			addr, err := s.port(ctx, svc, 9100)
			if err != nil {
				return err
			}
			if got[i], err = metric(addr, "werbot_tunnel_streams"); err != nil {
				return err
			}
		}
		if got != [2]int64{1, 1} {
			return fmt.Errorf("werbot_tunnel_streams server1=%d server2=%d, want 1 and 1", got[0], got[1])
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("placement: %w", err)
	}
	return nil
}

// readyAddr is where /readyz of a service is published: server HTTP only through nginx.
var readyAddr = map[string]struct {
	service string
	port    int
}{
	"server1": {"nginx", 8081}, "server2": {"nginx", 8082},
	"web1": {"web1", 8090}, "web2": {"web2", 8090},
	"gw1": {"gw1", 9090}, "gw2": {"gw2", 9090},
}

// ready returns an error naming the first service whose /readyz is not 200.
func (s *Stand) ready(ctx context.Context, services ...string) error {
	for _, svc := range services {
		ra := readyAddr[svc]
		addr, err := s.port(ctx, ra.service, ra.port)
		if err != nil {
			return err
		}
		if code := Ready(addr); code != http.StatusOK {
			return fmt.Errorf("%s /readyz: %d", svc, code)
		}
	}
	return nil
}

// poll calls cond with the deadline context every 500 ms until it returns nil
// or timeout passes; then it returns cond's last error.
func poll(ctx context.Context, timeout time.Duration, cond func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		err := cond(ctx)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("after %s: %w", timeout, err)
		case <-tick.C:
		}
	}
}
