package tunnel

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func TestConnectPushesSnapshot(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	s1 := uuid.New()
	c := g.connect(t, &gatewayv1.Heartbeat{Version: "1.0", Connectors: []string{"ssh"}, ActiveSessionIds: []string{s1.String()}})
	snap := c.next(t, 2*time.Second)
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(snap.GetVersion()) || snap.GetVersion() != snapshotVersion(snap) {
		t.Fatalf("version %q", snap.GetVersion())
	}
	eventually(t, func() (ok bool) {
		b.f.with(func(f *fake) { ok = len(f.hbs) > 0 && len(f.missing) > 0 })
		return ok
	}, "heartbeat processing")
	b.f.with(func(f *fake) {
		if h := f.hbs[0]; h.instanceID != g.InstanceID || h.version != "1.0" || !slices.Equal(h.connectors, []string{"ssh"}) {
			t.Fatalf("Heartbeat(%+v)", h)
		}
		if m := f.missing[0]; m.orgID != b.org || m.instanceID != g.InstanceID || !slices.Equal(m.active, []uuid.UUID{s1}) || m.reason != "gateway_lost" {
			t.Fatalf("EndMissing(%+v)", m)
		}
	})
	body := b.metric(t)
	if !hasLine(body, "werbot_tunnel_streams 1") {
		t.Fatalf("metrics:\n%s", body)
	}
	if n := metricValue(body, "werbot_snapshot_pushes_total"); n < 1 {
		t.Fatalf("werbot_snapshot_pushes_total = %d", n)
	}
	c.close()
	eventually(t, func() bool { return hasLine(b.metric(t), "werbot_tunnel_streams 0") }, "streams gauge 0")
}

func metricValue(body, name string) int {
	for l := range strings.Lines(body) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), name+" "); ok {
			n, _ := strconv.Atoi(v)
			return n
		}
	}
	return -1
}

func TestStreamMaxAge(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.maxAge = 300 * time.Millisecond })
	start := time.Now()
	c := b.instance(t, b.org, uuid.New()).connect(t, nil)
	err := c.closed(t, 2*time.Second)
	age := time.Since(start)
	wantCode(t, err, connect.CodeUnavailable, "rebalance")
	if age < 240*time.Millisecond || age > 460*time.Millisecond {
		t.Fatalf("stream lived %v, want 240ms..460ms", age)
	}
}

func TestHeartbeatTimeout(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.hbTimeout = 200 * time.Millisecond })
	c := b.instance(t, b.org, uuid.New()).connect(t, nil)
	c.next(t, 2*time.Second)
	wantCode(t, c.closed(t, 2*time.Second), connect.CodeUnavailable, "heartbeat timeout")

	// Regular heartbeats keep the stream open.
	c = b.instance(t, b.org, uuid.New()).connect(t, nil)
	c.next(t, 2*time.Second)
	for range 6 {
		time.Sleep(100 * time.Millisecond)
		if err := c.send(&gatewayv1.Heartbeat{Version: "1.0"}); err != nil {
			t.Fatal(err)
		}
	}
	c.quiet(t, 50*time.Millisecond)
}

func TestShutdown(t *testing.T) {
	b := newBench(t, nil)
	conns := []*conn{b.instance(t, b.org, uuid.New()).connect(t, nil), b.instance(t, uuid.New(), uuid.New()).connect(t, nil)}
	for _, c := range conns {
		c.next(t, 2*time.Second)
	}
	b.hub.Shutdown()
	for _, c := range conns {
		wantCode(t, c.closed(t, time.Second), connect.CodeUnavailable, "shutdown")
	}
	wantCode(t, b.instance(t, b.org, uuid.New()).connect(t, nil).closed(t, time.Second), connect.CodeUnavailable, "shutdown")
}

// countingRT counts response body bytes and records the stream content encoding.
type countingRT struct {
	next     http.RoundTripper
	n        atomic.Int64
	encoding atomic.Value
}

func (c *countingRT) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := c.next.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	c.encoding.Store(resp.Header.Get("Connect-Content-Encoding"))
	resp.Body = &countingBody{ReadCloser: resp.Body, n: &c.n}
	return resp, nil
}

type countingBody struct {
	io.ReadCloser
	n *atomic.Int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n.Add(int64(n))
	return n, err
}

func TestSnapshotCompressed(t *testing.T) {
	b := newBench(t, nil)
	b.f.with(func(f *fake) {
		f.snapshot = func(_, _ uuid.UUID) *gatewayv1.Snapshot {
			s := &gatewayv1.Snapshot{CedarPolicies: "permit(principal, action, resource);"}
			for i := range 2000 {
				id, _ := uuid.NewV7()
				s.Resources = append(s.Resources, &gatewayv1.Resource{Id: id.String(), Name: fmt.Sprintf("server-%04d", i), Type: "ssh",
					ConfigJson: fmt.Appendf(nil, `{"host":"10.0.%d.%d","port":22,"login":"root"}`, i/256, i%256)})
			}
			return s
		}
	})
	rt := &countingRT{}
	g := b.instanceWith(t, b.org, uuid.New(), func(next http.RoundTripper) http.RoundTripper { rt.next = next; return rt })
	snap := g.connect(t, nil).next(t, 5*time.Second)
	if len(snap.GetResources()) != 2000 {
		t.Fatalf("resources = %d", len(snap.GetResources()))
	}
	if enc, _ := rt.encoding.Load().(string); enc != "gzip" {
		t.Fatalf("Connect-Content-Encoding = %q, want gzip", enc)
	}
	if n, size := rt.n.Load(), proto.Size(snap); n >= int64(size/2) {
		t.Fatalf("wire bytes %d, snapshot %d: want < half", n, size)
	}
}
