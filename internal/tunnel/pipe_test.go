package tunnel

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// Stream hands the relay a pipe of the calling gateway and ticket: frames
// pass both ways, writes split into 32 KiB frames and a resize out of bounds
// is skipped while the stream lives.
func TestStreamBridges(t *testing.T) {
	b, rt := newRealtimeBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	ticket := uuid.New()
	st := dialStream(t, g, openFrame(ticket.String()))
	c := rt.nextBridge(t)
	if c.gw.ID != g.ID || c.gw.InstanceID != g.InstanceID || c.gw.OrgID != b.org || c.ticket != ticket {
		t.Fatalf("Bridge(%+v, %s), want the caller and the ticket", c.gw, c.ticket)
	}
	streamUp(t, st, c.p)
	streamDown(t, st, c.p)
	streamClose(t, st, c)
}

// streamUp checks gateway frames reach Read; out-of-bounds resizes and empty frames are skipped.
func streamUp(t *testing.T, st *streamClient, p Pipe) {
	t.Helper()
	for _, f := range []*gatewayv1.StreamFrame{dataFrame([]byte("ls\r")), resizeFrame(9999, 40), resizeFrame(100, 0), {}, resizeFrame(100, 30)} {
		if err := st.Send(&gatewayv1.StreamRequest{Frame: f}); err != nil {
			t.Fatal(err)
		}
	}
	if data, _, _, err := p.Read(); err != nil || string(data) != "ls\r" {
		t.Fatalf("Read = %q, %v; want ls", data, err)
	}
	if _, cols, rows, err := p.Read(); err != nil || cols != 100 || rows != 30 {
		t.Fatalf("Read = %dx%d, %v; want 100x30 after the skipped frames", cols, rows, err)
	}
}

// streamDown checks Write splits 100 KiB into four frames of at most 32 KiB and Resize passes.
func streamDown(t *testing.T, st *streamClient, p Pipe) {
	t.Helper()
	big := bytes.Repeat([]byte("x"), 100<<10)
	if err := p.Write(big); err != nil {
		t.Fatal(err)
	}
	var got []byte
	for i := range 4 {
		m, err := st.Receive()
		if err != nil || len(m.GetFrame().GetData()) > 32<<10 {
			t.Fatalf("data frame %d: %d bytes, %v", i, len(m.GetFrame().GetData()), err)
		}
		got = append(got, m.GetFrame().GetData()...)
	}
	if !bytes.Equal(got, big) {
		t.Fatalf("got %d bytes, want the 100 KiB written", len(got))
	}
	if err := p.Resize(120, 40); err != nil {
		t.Fatal(err)
	}
	if m, err := st.Receive(); err != nil || m.GetFrame().GetResize().GetCols() != 120 || m.GetFrame().GetResize().GetRows() != 40 {
		t.Fatalf("resize: %v, %v", m, err)
	}
}

// streamClose checks a gateway close reaches Read as a CloseReason, Close
// sends one close frame and nothing after it, and the stream ends with Bridge.
func streamClose(t *testing.T, st *streamClient, c bridgeCall) {
	t.Helper()
	if err := st.Send(&gatewayv1.StreamRequest{Frame: closeFrame("closed")}); err != nil {
		t.Fatal(err)
	}
	var r interface{ CloseReason() string }
	if _, _, _, err := c.p.Read(); !errors.As(err, &r) || r.CloseReason() != "closed" {
		t.Fatalf("Read after close = %v, want a CloseReason closed", err)
	}
	for range 2 {
		if err := c.p.Close("denied"); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.p.Write([]byte("late")); err == nil {
		t.Fatal("Write after Close succeeded")
	}
	if m, err := st.Receive(); err != nil || m.GetFrame().GetClose().GetReason() != "denied" {
		t.Fatalf("close: %v, %v", m, err)
	}
	c.done <- nil
	if m, err := st.Receive(); !errors.Is(err, io.EOF) {
		t.Fatalf("after Bridge: %v, %v; want one close frame, then the end", m, err)
	}
}

// A frame over the 64 KiB Stream read limit ends that stream; the other
// procedures keep their 4 MiB limit.
func TestStreamReadLimit(t *testing.T) {
	b, rt := newRealtimeBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	st := dialStream(t, g, openFrame(uuid.NewString()))
	c := rt.nextBridge(t)
	if err := st.Send(&gatewayv1.StreamRequest{Frame: dataFrame(make([]byte, 65<<10))}); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := c.p.Read()
	wantCode(t, err, connect.CodeResourceExhausted, "")
	c.done <- err
}

func TestStreamRequiresOpenFirst(t *testing.T) {
	b, rt := newRealtimeBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	for _, first := range []*gatewayv1.StreamFrame{dataFrame([]byte("ls")), openFrame("x")} {
		st := dialStream(t, g, first)
		_, err := st.Receive()
		wantCode(t, err, connect.CodeInvalidArgument, "")
	}
	if len(rt.bridges) != 0 {
		t.Fatal("Bridge called without a valid open frame")
	}

	plain := newBench(t, nil) // no relay
	st := dialStream(t, plain.instance(t, plain.org, uuid.New()), openFrame(uuid.NewString()))
	_, err := st.Receive()
	wantCode(t, err, connect.CodeUnimplemented, "")
}
