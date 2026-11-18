package tunnel

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// bigSnapshot is a deterministic snapshot of about 200 KB; tag changes its content.
func bigSnapshot(tag string) *gatewayv1.Snapshot {
	s := &gatewayv1.Snapshot{CedarPolicies: "permit(principal, action, resource); // " + tag}
	for i := range 2000 {
		s.Resources = append(s.Resources, &gatewayv1.Resource{Id: fmt.Sprintf("00000000-0000-7000-8000-%012d", i), Name: fmt.Sprintf("server-%04d", i),
			Type: "ssh", ConfigJson: fmt.Appendf(nil, `{"host":"10.0.%d.%d","port":22,"login":"root"}`, i/256, i%256)})
	}
	return s
}

func stdGzip(t testing.TB, in []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(in); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// gzipOnceRun compresses in as connect does: Reset, Write, Close.
func gzipOnceRun(c *gzipCache, in []byte) ([]byte, error) {
	var out bytes.Buffer
	z := &gzipOnce{c: c}
	z.Reset(&out)
	if _, err := z.Write(in); err != nil {
		return nil, err
	}
	err := z.Close()
	z.Reset(io.Discard)
	return out.Bytes(), err
}

// runs is how many large messages c has gzipped while none was evicted: put
// appends a key per run, so a second run of one message repeats its key.
func (c *gzipCache) runs() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.order)
}

// Concurrent sends of one message gzip it once and all get the bytes the
// default gzip compressor writes; another message is compressed anew.
func TestGzipOnce(t *testing.T) {
	var c gzipCache
	in, err := proto.Marshal(bigSnapshot("a"))
	if err != nil {
		t.Fatal(err)
	}
	want := stdGzip(t, in)
	outs := make([][]byte, 50)
	var wg sync.WaitGroup
	for i := range outs {
		wg.Go(func() {
			var err error
			if outs[i], err = gzipOnceRun(&c, in); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	for i, out := range outs {
		if !bytes.Equal(out, want) {
			t.Fatalf("send %d: %d bytes differ from the default gzip output (%d bytes)", i, len(out), len(want))
		}
	}
	if n := c.runs(); n != 1 {
		t.Fatalf("gzip ran %d times for one message, want 1", n)
	}

	other := append(bytes.Clone(in), 'x')
	out, err := gzipOnceRun(&c, other)
	if err != nil || !bytes.Equal(out, stdGzip(t, other)) || c.runs() != 2 {
		t.Fatalf("another message: err %v, runs %d", err, c.runs())
	}
	for i := range gzipCacheSize + 1 {
		if _, err := gzipOnceRun(&c, fmt.Appendf(bytes.Clone(in), "%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(c.out); n != gzipCacheSize || len(c.order) != gzipCacheSize {
		t.Fatalf("cache holds %d entries, want %d", n, gzipCacheSize)
	}

	// A message under gzipCacheMin is gzipped every time (it is never cached) and evicts nothing.
	small := in[:gzipCacheMin-1]
	oldest := c.order[0]
	for range 2 {
		if out, err := gzipOnceRun(&c, small); err != nil || !bytes.Equal(out, stdGzip(t, small)) {
			t.Fatalf("small message: err %v", err)
		}
	}
	if _, cached := c.get(sha256.Sum256(small)); cached || c.order[0] != oldest || len(c.out) != gzipCacheSize {
		t.Fatalf("small message: cached %v, cache %d entries, oldest evicted %v", cached, len(c.out), c.order[0] != oldest)
	}
}

// teeRT keeps the response body of every Connect stream.
type teeRT struct {
	next   http.RoundTripper
	mu     sync.Mutex
	bodies []*syncBuffer
}

func (r *teeRT) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	buf := &syncBuffer{}
	r.mu.Lock()
	r.bodies = append(r.bodies, buf)
	r.mu.Unlock()
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.TeeReader(resp.Body, buf), resp.Body}
	return resp, nil
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.b.Bytes())
}

// firstFrame returns the first enveloped message of a Connect stream body.
func firstFrame(t *testing.T, body []byte) []byte {
	t.Helper()
	if len(body) < 5 || len(body) < 5+int(binary.BigEndian.Uint32(body[1:5])) {
		t.Fatalf("stream body of %d bytes holds no whole message", len(body))
	}
	return body[:5+binary.BigEndian.Uint32(body[1:5])]
}

// The streams of one pool get the same snapshot bytes on the wire, and the
// server gzips each snapshot once for all of them.
func TestPoolStreamsShareEncodedSnapshot(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 20 * time.Millisecond })
	var tag atomic.Value
	tag.Store("v1")
	b.f.with(func(f *fake) {
		f.snapshot = func(_, _ uuid.UUID) *gatewayv1.Snapshot { return bigSnapshot(tag.Load().(string)) }
	})
	pool := uuid.New()
	rts := make([]*teeRT, 5)
	conns := make([]*conn, len(rts))
	for i := range rts {
		rts[i] = &teeRT{}
		g := b.instanceWith(t, b.org, pool, func(next http.RoundTripper) http.RoundTripper { rts[i].next = next; return rts[i] })
		conns[i] = g.connect(t, nil)
		conns[i].next(t, 5*time.Second)
	}
	if n := b.hub.h.gz.runs(); n != 1 {
		t.Fatalf("gzip ran %d times for %d streams of one snapshot, want 1", n, len(conns))
	}
	var want []byte
	for i, rt := range rts {
		rt.mu.Lock()
		body := rt.bodies[0]
		rt.mu.Unlock()
		frame := firstFrame(t, body.bytes())
		if frame[0]&1 == 0 {
			t.Fatalf("stream %d: snapshot not compressed", i)
		}
		if want == nil {
			want = frame
		} else if !bytes.Equal(frame, want) {
			t.Fatalf("stream %d got other snapshot bytes", i)
		}
	}

	tag.Store("v2")
	b.hub.OrgChanged(b.org.String())
	for _, c := range conns {
		if !proto.Equal(c.next(t, 5*time.Second).GetResources()[0], bigSnapshot("v2").GetResources()[0]) {
			t.Fatal("wrong snapshot after the change")
		}
	}
	if n := b.hub.h.gz.runs(); n != 2 {
		t.Fatalf("gzip ran %d times for two snapshots, want 2", n)
	}
}
