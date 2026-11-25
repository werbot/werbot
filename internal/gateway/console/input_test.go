package console

import (
	"context"
	"io"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
)

// sleeper is a line that runs until its channel closes, reporting the signals it gets.
func sleeper(sigs *[]string, exitOnInt bool) targetFunc {
	return func(ctx context.Context, _ string, _ io.Writer, sig <-chan string) (int, error) {
		for {
			select {
			case s := <-sig:
				*sigs = append(*sigs, s)
				if exitOnInt {
					return 130, nil
				}
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
	}
}

const sleepScript = "cd -- '/home/u' || exit 1\nsleep 9"

func TestConsoleCtrlCAtPrompt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, nil).run()
		h.typ("ls\r", "ab\x03")
		h.endsWith("alice@db:~$ ab^C\r\nalice@db:~$ ")
		h.running()
		h.typ("\x1b[A\r") // the previous line from the history
		require.Equal(t, []string{"ls", "ls"}, h.commands())
	})
}

func TestConsoleCtrlCDuringExec(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var sigs []string
		h := newHarness(t, answering(map[string]targetFunc{sleepScript: sleeper(&sigs, true)}), nil).run()
		h.typ("sleep 9\rabc\r", "xyz\r")
		h.running()
		h.typ("\x03")
		require.Equal(t, []string{"INT"}, sigs)
		h.endsWith("sleep 9\r\n[130] alice@db:~$ ")
		h.typ("\r")
		require.Equal(t, []string{"sleep 9"}, h.commands(), "input typed during the line is dropped")
	})
}

// Ruling R4: a second Ctrl-C during the same line closes its channel.
func TestConsoleSecondCtrlCCloses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var sigs []string
		h := newHarness(t, answering(map[string]targetFunc{sleepScript: sleeper(&sigs, false)}), nil).run()
		h.typ("sleep 9\r", "\x03")
		require.Equal(t, []string{"INT"}, sigs)
		h.endsWith("sleep 9\r\n")
		h.typ("\x03")
		h.endsWith("sleep 9\r\n^C\r\n[130] alice@db:~$ ")
		h.running()
	})
}

func TestConsoleCtrlCDuringApproval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, func(ctx context.Context, _ string) (gateway.CommandCheck, error) {
			<-ctx.Done()
			return gateway.CommandCheck{}, ctx.Err()
		}).run()
		h.typ("rm x\r", "\x03")
		h.endsWith("rm x\r\n^C\r\n[130] alice@db:~$ ")
		_, execs, _ := h.calls()
		require.Equal(t, []string{startCommand}, execs)
	})
}

// Ruling G28: a line with a control character or over 4096 bytes runs nothing.
func TestConsoleRefusesControlCharacters(t *testing.T) {
	for name, line := range map[string]string{
		"C1 control": "ls\u0085x",
		"too long":   strings.Repeat("é", 2100),
		"editor cap": strings.Repeat("a", 5000), // term keeps 4096 runes of it
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t, nil, nil).run()
				h.typ(line + "\r")
				h.endsWith("\r\nCommand denied: control characters or line too long\r\n[1] alice@db:~$ ")
				require.Empty(t, h.commands())
			})
		})
	}
}

// Ruling G28: bracketed-paste markers, also split across reads, never reach the line editor.
func TestConsolePasteMarkersStripped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, nil).run()
		h.typ("\x1b[20", "0~ls", " -l\x1b[2", "01~\r")
		h.typ("\x1b[200~\x1b[200~pwd -P\x1b[201~\r")
		require.Equal(t, []string{"ls -l", "pwd -P"}, h.commands())
	})
}

// Ruling G29: an escape sequence that fills the line editor's buffer ends the console.
func TestConsoleEscapeOverflow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t, nil, nil).run()
		h.typ("\x1b[" + strings.Repeat("1", 300))
		require.NoError(t, h.ended())
		h.endsWith("Console input error\r\n")
		require.Empty(t, h.commands())
	})
}
