package telnetconnector

import (
	"context"
	"io"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/werbot/werbot/internal/gateway"
)

// The current window is not sent again: a client NAWS that repeats the window
// applied before the relay (its TTYPE answered first) changes nothing.
func TestRelaySkipsCurrentWindow(t *testing.T) {
	local, peer := pipePair(t)
	tc := NewClient(local, "xterm")
	inR, inW := io.Pipe()
	defer inW.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	g := gateway.NewGuard(cancel, gateway.Secret{})
	rec := &resizeRec{}
	resize := make(chan gateway.Window, 2)
	resize <- gateway.Window{Cols: 132, Rows: 40}
	resize <- gateway.Window{Cols: 100, Rows: 30}
	close(resize)

	done := make(chan error, 1)
	go func() { done <- relay(ctx, g, tc, inR, io.Discard, rec, resize, gateway.Window{Cols: 132, Rows: 40}) }()
	deadline := time.Now().Add(testDeadline)
	for len(rec.get()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	_ = peer.Close()
	if err := <-done; err != nil {
		t.Fatalf("relay: %v", err)
	}
	if err := g.Wait(); err != nil {
		t.Fatalf("guard: %v", err)
	}
	if got := rec.get(); !slices.Equal(got, [][2]int{{100, 30}}) {
		t.Fatalf("resizes = %v, want only 100x30", got)
	}
}

type resizeRec struct {
	mu    sync.Mutex
	sizes [][2]int
}

func (*resizeRec) Output([]byte)               {}
func (*resizeRec) Input([]byte)                {}
func (*resizeRec) Marker(string)               {}
func (*resizeRec) Event(any) error             { return nil }
func (*resizeRec) Close(context.Context) error { return nil }
func (r *resizeRec) Resize(cols, rows int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sizes = append(r.sizes, [2]int{cols, rows})
}

func (r *resizeRec) get() [][2]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.sizes)
}
