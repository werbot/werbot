//go:build cluster || load

package cluster

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

var probe = &http.Client{Timeout: 5 * time.Second}

// Ready is the status code of GET http://addr/readyz; 0 when there is no answer.
func Ready(addr string) int {
	res, err := probe.Get("http://" + addr + "/readyz")
	if err != nil {
		return 0
	}
	_ = res.Body.Close()
	return res.StatusCode
}

// Metric reads the series name{labels} from http://addr/metrics; labels are
// key, value pairs in registration order. A missing series fails the test: werbot
// exports every series the stand reads at startup, so absence means a wrong name or label.
func Metric(t testing.TB, addr, name string, labels ...string) int64 {
	t.Helper()
	v, err := ReadMetric(addr, name, labels...)
	require.NoError(t, err)
	return v
}

// ReadMetric is Metric without a test: an error instead of a failure, so
// background goroutines and polling conditions can call it.
func ReadMetric(addr, name string, labels ...string) (int64, error) {
	series := name
	if len(labels) > 0 {
		pairs := make([]string, 0, len(labels)/2)
		for i := 0; i+1 < len(labels); i += 2 {
			pairs = append(pairs, fmt.Sprintf("%s=%q", labels[i], labels[i+1]))
		}
		series += "{" + strings.Join(pairs, ",") + "}"
	}
	res, err := probe.Get("http://" + addr + "/metrics")
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("GET %s/metrics: %s", addr, res.Status)
	}
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), series+" "); ok {
			return strconv.ParseInt(v, 10, 64)
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("%s: no series %s", addr, series)
}

// connectDB opens a pool to the stand's Postgres. Every new connection resolves
// the published port again: it changes when the postgres container restarts.
func (s *Stand) connectDB(ctx context.Context) error {
	cfg, err := pgxpool.ParseConfig("postgres://werbot:werbot@postgres/werbot?sslmode=disable")
	if err != nil {
		return err
	}
	// "postgres" is a placeholder host: no DNS lookup, DialFunc picks the address
	cfg.ConnConfig.LookupFunc = func(_ context.Context, host string) ([]string, error) { return []string{host}, nil }
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, _ string) (net.Conn, error) {
		addr, err := s.port(ctx, "postgres", 5432)
		if err != nil {
			return nil, err
		}
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
	s.DB, err = pgxpool.NewWithConfig(ctx, cfg)
	return err
}

// LastSession is the newest session of resourceID: its id, client IP and the hostname of its gateway instance.
func (s *Stand) LastSession(t testing.TB, resourceID uuid.UUID) (id uuid.UUID, clientIP netip.Addr, instance string) {
	t.Helper()
	var ip string
	err := s.DB.QueryRow(context.Background(), `
		SELECT s.id, host(s.client_ip), coalesce(gi.hostname, '')
		FROM session s LEFT JOIN gateway_instance gi ON gi.id = s.gateway_instance_id
		WHERE s.resource_id = $1 ORDER BY s.started_at DESC, s.id DESC LIMIT 1`, resourceID).Scan(&id, &ip, &instance)
	require.NoError(t, err)
	clientIP, err = netip.ParseAddr(ip)
	require.NoError(t, err)
	return id, clientIP, instance
}

// OpenSessionOn is the newest open session served by the gateway instance hostname.
func (s *Stand) OpenSessionOn(t testing.TB, hostname string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := s.DB.QueryRow(context.Background(), `
		SELECT s.id FROM session s JOIN gateway_instance gi ON gi.id = s.gateway_instance_id
		WHERE gi.hostname = $1 AND s.ended_at IS NULL ORDER BY s.started_at DESC, s.id DESC LIMIT 1`, hostname).Scan(&id)
	require.NoError(t, err, "open session on %s", hostname)
	return id
}

// WaitEnded polls GetSession through Edge every second until session id has
// ended, asserts its end reason and returns how long the wait took.
func (s *Stand) WaitEnded(t testing.TB, id uuid.UUID, reason string, timeout time.Duration) time.Duration {
	t.Helper()
	start := time.Now()
	var got atomic.Pointer[werbotv1.Session] // Eventually runs the condition in its own goroutine
	ended := func() bool {
		res, err := s.Edge.Sessions.GetSession(context.Background(), connect.NewRequest(&werbotv1.SessionServiceGetSessionRequest{Id: id.String()}))
		if err != nil {
			return false
		}
		got.Store(res.Msg.GetSession())
		return res.Msg.GetSession().GetEndedAt() != nil
	}
	if !assert.Eventually(t, ended, timeout, time.Second) {
		t.Fatalf("session %s not ended within %s: %v", id, timeout, got.Load())
	}
	elapsed := time.Since(start)
	require.Equal(t, reason, got.Load().GetEndReason(), "end reason of session %s", id)
	return elapsed
}

// WaitVerified polls VerifyRecording through Edge every second until the recording of session id verifies.
func (s *Stand) WaitVerified(t testing.TB, id uuid.UUID, timeout time.Duration) {
	t.Helper()
	var last atomic.Value // last reason; Eventually runs the condition in its own goroutine
	verified := func() bool {
		res, err := s.Edge.Sessions.VerifyRecording(context.Background(), connect.NewRequest(&werbotv1.VerifyRecordingRequest{SessionId: id.String()}))
		if err != nil {
			last.Store(err.Error())
			return false
		}
		last.Store(res.Msg.GetReason())
		return res.Msg.GetVerified()
	}
	if !assert.Eventually(t, verified, timeout, time.Second) {
		t.Fatalf("recording of session %s not verified within %s: %v", id, timeout, last.Load())
	}
}
