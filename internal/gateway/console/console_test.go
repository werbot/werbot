package console

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
)

const banner = "Werbot command console on db. Type help for details.\r\n"

func TestConsoleStartsAtHome(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, nil).run()
		_, execs, _ := h.calls()
		require.Equal(t, []string{"cd && pwd -P"}, execs)
		require.Equal(t, banner+"alice@db:~$ ", h.screen.String())
		h.running()
	})
}

// Rulings G27, G30, G31: a start answer that is not one clean absolute line
// ends the console with ErrConsoleUnavailable, which the front prints once.
func TestConsoleStartFailure(t *testing.T) {
	for name, target := range map[string]targetFunc{
		"exit code":     output("/home/u\n", 1),
		"empty":         output("", 0),
		"relative":      output("rel/path\n", 0),
		"two lines":     output("/a\n/b\n", 0),
		"not clean":     output("/a/../b\n", 0),
		"control":       output("/a\x1bb\n", 0),
		"too long":      output("/"+strings.Repeat("a", 1024)+"\n", 0),
		"invalid utf-8": output("/a\xff\n", 0),
		"exec error": func(context.Context, string, io.Writer, <-chan string) (int, error) {
			return 0, errors.New("channel failed")
		},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t, target, nil).run()
				require.ErrorIs(t, h.ended(), gateway.ErrConsoleUnavailable)
				require.Empty(t, h.screen.String())
				require.Empty(t, h.commands())
			})
		})
	}
}

func TestConsoleRequiresChecker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, nil)
		h.s.CheckCommand = nil
		h.run()
		require.Error(t, h.ended())
		_, execs, _ := h.calls()
		require.Empty(t, execs)
	})
}

func TestConsoleRunsInCheckedCwd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, answering(map[string]targetFunc{
			"cd -- '/home/u' || exit 1\nls -la": output("a\nb\n", 0),
		}), nil).run()
		h.typ("ls -la\r")
		checks, execs, _ := h.calls()
		require.Equal(t, []checkCall{{"ls -la", "/home/u", "/home/u"}}, checks)
		require.Equal(t, []string{startCommand, "cd -- '/home/u' || exit 1\nls -la"}, execs)
		require.Equal(t, banner+"alice@db:~$ ls -la\r\na\r\nb\r\nalice@db:~$ ", h.screen.String())
		_, _, markers, _ := h.rec.snapshot()
		require.Equal(t, []string{"ls -la [read]"}, markers)
	})
}

func TestConsoleDenied(t *testing.T) {
	for want, err := range map[string]error{
		"Command denied: path outside allowed folders": &gateway.CommandDeniedError{Err: gateway.ErrDenied, Message: "path outside allowed folders"},
		"Gateway is not ready, try again later":        gateway.ErrNotReady,
		"Approval timed out":                           gateway.ErrApprovalTimeout,
		"Access denied":                                errors.New("internal detail"),
	} {
		t.Run(want, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t, nil, func(context.Context, string) (gateway.CommandCheck, error) {
					return gateway.CommandCheck{}, err
				}).run()
				h.typ("rm x\r")
				_, execs, _ := h.calls()
				require.Equal(t, []string{startCommand}, execs)
				h.endsWith("rm x\r\n" + want + "\r\n[1] alice@db:~$ ")
				_, _, markers, _ := h.rec.snapshot()
				require.Empty(t, markers)
			})
		})
	}
}

func TestConsoleExitCodeInPrompt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, answering(map[string]targetFunc{
			"cd -- '/home/u' || exit 1\nfalse": output("", 3),
		}), nil).run()
		h.typ("false\r")
		h.endsWith("[3] alice@db:~$ ")
		h.typ("true\r")
		h.endsWith("true\r\nalice@db:~$ ")
	})
}

// Ruling G32: builtins are exact words; other forms run on the target after the check.
func TestConsoleBuiltins(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, nil).run()
		h.typ("pwd\r")
		h.endsWith("pwd\r\n/home/u\r\nalice@db:~$ ")
		h.typ("help\r")
		h.endsWith("help\r\n" + strings.ReplaceAll(helpText, "\n", "\r\n") + "alice@db:~$ ")
		require.Empty(t, h.commands())
		h.typ("exit 3\r", "pwd -L\r")
		require.Equal(t, []string{"exit 3", "pwd -L"}, h.commands())
		_, execs, _ := h.calls()
		require.Equal(t, "cd -- '/home/u' || exit 1\npwd -L", execs[len(execs)-1])
		h.typ("exit\r")
		require.NoError(t, h.ended())
		_, _, exit := h.calls()
		require.Equal(t, []int{0}, exit)
	})
	synctest.Test(t, func(t *testing.T) { // Ctrl-D on an empty line
		h := newHarness(t, nil, nil).run()
		h.typ("\x04")
		require.NoError(t, h.ended())
		_, _, exit := h.calls()
		require.Equal(t, []int{0}, exit)
	})
}

func TestConsoleSessionResult(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, func(_ context.Context, command string) (gateway.CommandCheck, error) {
			switch command {
			case "ls":
				return gateway.CommandCheck{Classes: []string{"read"}, Paths: []string{"/home/u"}}, nil
			case "cp a b":
				return gateway.CommandCheck{Classes: []string{"read", "write"}, Paths: []string{"/home/u/b", "/home/u/a"}}, nil
			case "rm x":
				return gateway.CommandCheck{}, &gateway.CommandDeniedError{Err: gateway.ErrDenied}
			}
			return gateway.CommandCheck{Classes: []string{"egress"}, Paths: []string{"/tmp"}}, nil
		}).run()
		h.typ("ls\r", "cp a b\r", "rm x\r", "cd /tmp\r", "pwd\r")
		require.Equal(t, "cp a b", h.s.Command)
		require.Equal(t, []string{"read", "write"}, h.s.Classes)
		require.Equal(t, []string{"/home/u", "/home/u/a", "/home/u/b"}, h.s.Paths)
	})
}

func TestConsoleCtxCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) { // at the prompt
		h := newHarness(t, nil, nil).run()
		h.typ("ls")
		h.cancel()
		require.NoError(t, h.ended())
	})
	synctest.Test(t, func(t *testing.T) { // during a line
		h := newHarness(t, answering(map[string]targetFunc{
			"cd -- '/home/u' || exit 1\nsleep 9": func(ctx context.Context, _ string, _ io.Writer, _ <-chan string) (int, error) {
				<-ctx.Done()
				return 0, ctx.Err()
			},
		}), nil).run()
		h.typ("sleep 9\r")
		h.running()
		h.cancel()
		require.NoError(t, h.ended())
	})
}

// A home of "/" is never folded into "~".
func TestConsoleRootHomeFolding(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, home("/"), nil).run()
		h.endsWith("alice@db:/$ ")
		h.typ("cd /etc\r")
		h.endsWith("alice@db:/etc$ ")
	})
	for _, c := range []struct{ cwd, home, want string }{
		{"/home/u", "/home/u", "~"},
		{"/home/u/app", "/home/u", "~/app"},
		{"/home/uu", "/home/u", "/home/uu"},
		{"/etc", "/", "/etc"},
		{"/", "/", "/"},
	} {
		require.Equal(t, c.want, fold(c.cwd, c.home), c)
	}
}

func TestConsoleRecordsEverything(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, answering(map[string]targetFunc{
			"cd -- '/home/u' || exit 1\nls": output("a\nb\n", 0),
		}), func(_ context.Context, command string) (gateway.CommandCheck, error) {
			if command == "rm x" {
				return gateway.CommandCheck{}, &gateway.CommandDeniedError{Err: gateway.ErrDenied}
			}
			return allowRead(context.Background(), command)
		}).run()
		h.typ("ls\r", "rm x\r")
		h.resize <- gateway.Window{Cols: 100, Rows: 30}
		synctest.Wait()
		long := strings.Repeat("x", 100-len("[1] alice@db:~$ ")-1)
		h.typ(long)
		h.endsWith("[1] alice@db:~$ " + long) // no wrap at the old width of 80
		out, in, _, resizes := h.rec.snapshot()
		require.Equal(t, h.screen.String(), out)
		require.Contains(t, out, banner+"alice@db:~$ ls\r\na\r\nb\r\n")
		require.Contains(t, out, "Access denied\r\n")
		require.Equal(t, "ls\rrm x\r"+long, in)
		require.Equal(t, []gateway.Window{{Cols: 100, Rows: 30}}, resizes)
		h.cancel()
		require.NoError(t, h.ended())
		h.typ("late")
		_, in, _, _ = h.rec.snapshot()
		require.Equal(t, "ls\rrm x\r"+long, in, "input after Run returned is not recorded")
	})
}

// Ruling G34: a terminal without a PTY gets the console too.
func TestConsoleWithoutPTY(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, nil)
		h.s.Terminal.Term, h.s.Terminal.Window = "", gateway.Window{}
		h.run()
		h.typ("ls\n")
		require.Equal(t, []string{"ls"}, h.commands())
		h.endsWith("ls\r\nalice@db:~$ ")
	})
}
