package console

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
)

func TestConsoleCd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, answering(map[string]targetFunc{
			"cd -- '/home/u/app' && pwd -P": output("/srv/app\n", 0),
		}), nil).run()
		h.typ("cd app\r")
		checks, execs, _ := h.calls()
		require.Equal(t, []checkCall{
			{"cd -- '/home/u/app'", "/home/u", "/home/u"},
			{"cd -- '/srv/app'", "/home/u", "/home/u"},
		}, checks)
		require.Equal(t, "cd -- '/home/u/app' && pwd -P", execs[1])
		h.endsWith("cd app\r\nalice@db:/srv/app$ ")
		_, _, markers, _ := h.rec.snapshot()
		require.Equal(t, []string{"cd -- '/srv/app' [read]"}, markers)
		h.typ("ls\r")
		checks, execs, _ = h.calls()
		require.Equal(t, checkCall{"ls", "/srv/app", "/home/u"}, checks[2])
		require.Equal(t, "cd -- '/srv/app' || exit 1\nls", execs[2])
	})
}

func TestConsoleCdResolvedOutsideDenied(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, answering(map[string]targetFunc{
			"cd -- '/home/u/app' && pwd -P": output("/srv/app\n", 0),
		}), func(_ context.Context, command string) (gateway.CommandCheck, error) {
			if command == "cd -- '/srv/app'" {
				return gateway.CommandCheck{}, &gateway.CommandDeniedError{Err: gateway.ErrDenied, Message: "path outside allowed folders"}
			}
			return allowRead(context.Background(), command)
		}).run()
		h.typ("cd app\r")
		h.endsWith("Command denied: path outside allowed folders\r\n[1] alice@db:~$ ")
		_, _, markers, _ := h.rec.snapshot()
		require.Empty(t, markers)
		h.typ("ls\r")
		checks, _, _ := h.calls()
		require.Equal(t, checkCall{"ls", "/home/u", "/home/u"}, checks[len(checks)-1])
	})
}

func TestConsoleCdHomeAndTilde(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, nil).run()
		h.typ("cd ~/x\r")
		h.endsWith("alice@db:~/x$ ")
		h.typ("cd\r")
		h.endsWith("alice@db:~$ ")
		h.typ("cd ..\r")
		h.endsWith("alice@db:/home$ ")
		h.typ("cd /\r")
		h.endsWith("alice@db:/$ ")
		h.typ("cd ~\r")
		h.endsWith("alice@db:~$ ")
		require.Equal(t, []string{
			"cd -- '/home/u/x'", "cd -- '/home/u'", "cd -- '/home'", "cd -- '/'", "cd -- '/home/u'",
		}, h.commands())
	})
}

// Ruling G32: cd takes zero or one plain path; anything else is refused unchecked.
func TestConsoleCdRejectsShellSyntax(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, nil).run()
		for _, line := range []string{"cd $HOME", "cd 'a b'", "cd -", "cd -P", "cd a;b", "cd a b", "cd a*", "cd ~bob", "cd a~"} {
			h.typ(line + "\r")
			h.endsWith(line + "\r\ncd: use a plain path without quotes or expansions\r\n[1] alice@db:~$ ")
		}
		require.Empty(t, h.commands())
	})
}

// Ruling G31: a cd answer that is not one clean absolute line keeps the cwd.
func TestConsoleCdBadAnswer(t *testing.T) {
	for name, answer := range map[string]string{
		"relative":  "rel\n",
		"two lines": "/a\n/b\n",
		"not clean": "/a/./b\n",
		"control":   "/x\u0085\n",
		"too long":  "/" + strings.Repeat("a", 2000) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t, answering(map[string]targetFunc{
					"cd -- '/home/u/app' && pwd -P": output(answer, 0),
				}), nil).run()
				h.typ("cd app\r")
				h.endsWith("cd app\r\ncd: cannot use this directory\r\n[1] alice@db:~$ ")
				require.Equal(t, []string{"cd -- '/home/u/app'"}, h.commands())
				_, _, markers, _ := h.rec.snapshot()
				require.Empty(t, markers)
			})
		})
	}
	synctest.Test(t, func(t *testing.T) { // the target's cd fails
		h := newHarness(t, answering(map[string]targetFunc{
			"cd -- '/home/u/app' && pwd -P": output("", 2),
		}), nil).run()
		h.typ("cd app\r")
		h.endsWith("cd app\r\n[2] alice@db:~$ ")
	})
}

// A cwd with any printable bytes reaches the target as the same path.
func TestConsoleQuotesCwd(t *testing.T) {
	const dir = `/srv/it's a $dir "x" \ *`
	var script string
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, answering(map[string]targetFunc{
			"cd -- '/home/u/app' && pwd -P": output(dir+"\n", 0),
		}), nil).run()
		h.typ("cd app\r", "ls\r")
		_, execs, _ := h.calls()
		script = execs[len(execs)-1]
	})
	first, line, ok := strings.Cut(script, "\n")
	require.True(t, ok)
	require.Equal(t, "ls", line)
	quoted, ok := strings.CutSuffix(strings.TrimPrefix(first, "cd -- "), " || exit 1")
	require.True(t, ok)
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	got, err := exec.Command(sh, "-c", "printf %s "+quoted).Output() //nolint:gosec // the test's own script
	require.NoError(t, err)
	require.Equal(t, dir, string(got))
}
