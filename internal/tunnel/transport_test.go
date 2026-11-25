package tunnel

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// StartSession hands the transport of the request to Sessions.Start; a value
// outside tcp, ws, internal is InvalidArgument and starts nothing.
func TestStartSessionTransport(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	b.f.with(func(f *fake) { f.owned = true })
	user, res := uuid.NewString(), uuid.NewString()
	start := func(transport string) error {
		_, err := g.client.StartSession(ctx, connect.NewRequest(&gatewayv1.StartSessionRequest{
			UserId: user, ResourceId: res, ClientIp: "10.1.2.3", Kind: "terminal", Transport: transport}))
		return err
	}

	_, err := g.client.StartSession(ctx, connect.NewRequest(&gatewayv1.StartSessionRequest{
		UserId: user, ResourceId: res, ClientIp: "10.1.2.3", Kind: "terminal", Transport: "bogus"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	b.f.with(func(f *fake) {
		if len(f.starts) != 0 {
			t.Fatalf("Start called for an unknown transport: %+v", f.starts)
		}
	})

	var got []string
	for _, transport := range []string{"", "ws", "internal", "tcp"} {
		if err := start(transport); err != nil {
			t.Fatalf("%q: %v", transport, err)
		}
	}
	b.f.with(func(f *fake) {
		for _, s := range f.starts {
			got = append(got, s.Transport)
		}
	})
	want := []string{"", "ws", "internal", "tcp"}
	if len(got) != len(want) {
		t.Fatalf("transports %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("transports %q, want %q", got, want)
		}
	}
}
