package tunnel

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// StartSession passes an exec session's command and classes to Sessions.Start;
// a session without them records "" and nil. Oversize commands and unknown or
// too many classes are rejected before Start.
func TestStartSessionCommand(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	b.f.with(func(f *fake) { f.owned = true })
	user, res := uuid.NewString(), uuid.NewString()
	start := func(command string, classes []string) error {
		_, err := g.client.StartSession(ctx, connect.NewRequest(&gatewayv1.StartSessionRequest{
			UserId: user, ResourceId: res, ClientIp: "10.1.2.3", Kind: "exec", Command: command, Classes: classes}))
		return err
	}

	if err := start("ls", []string{"read"}); err != nil {
		t.Fatal(err)
	}
	if err := start("", nil); err != nil {
		t.Fatal(err)
	}
	if err := start(strings.Repeat("a", 65536), []string{"read", "write", "destructive", "privilege", "secrets", "egress", "opaque"}); err != nil {
		t.Fatalf("command at the 64 KiB bound, all seven classes: %v", err)
	}
	b.f.with(func(f *fake) {
		if len(f.starts) != 3 {
			t.Fatalf("Start called %d times, want 3", len(f.starts))
		}
		if s := f.starts[0]; s.Command != "ls" || !slices.Equal(s.Classes, []string{"read"}) {
			t.Fatalf("exec session: command %q classes %q", s.Command, s.Classes)
		}
		if s := f.starts[1]; s.Command != "" || s.Classes != nil {
			t.Fatalf("no command: command %q classes %#v, want \"\" and nil", s.Command, s.Classes)
		}
	})

	// command over 64 KiB, unknown class, eight classes
	wantCode(t, start(strings.Repeat("a", 65537), []string{"read"}), connect.CodeInvalidArgument, "")
	wantCode(t, start("ls", []string{"shell"}), connect.CodeInvalidArgument, "")
	wantCode(t, start("ls", slices.Repeat([]string{"read"}, 8)), connect.CodeInvalidArgument, "")
	b.f.with(func(f *fake) {
		if len(f.starts) != 3 {
			t.Fatalf("Start called for an invalid request: %d calls", len(f.starts))
		}
	})
}
