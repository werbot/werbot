package console

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/werbot/werbot/internal/gateway"
)

const (
	maxLineBytes  = 4096
	maxPaths      = 256 // paths kept in the session result
	codeInterrupt = 130 // 128 + SIGINT
)

// errInterrupted cancels a check or a line on the user's Ctrl-C.
var errInterrupted = errors.New("console: interrupted")

// validLine refuses control characters, invalid UTF-8 and overlong lines
// (Ruling G28). A line at term's cap of 4096 runes may have lost its tail.
func validLine(line string) bool {
	return len(line) <= maxLineBytes && utf8.RuneCountInString(line) < maxLineBytes && utf8.ValidString(line) &&
		!strings.ContainsFunc(line, unicode.IsControl)
}

// run checks line and runs it on the target in the checked cwd (Ruling C20),
// marking it in the recording and in the session result.
func (c *console) run(ctx context.Context, line string) {
	chk, ok := c.check(ctx, line)
	if !ok {
		return
	}
	c.s.Recording.Marker(marker(line, chk.Classes))
	c.record(line, chk)
	c.last = c.runOnTarget(ctx, gateway.ConsoleScript(c.cwd, line), c.t)
}

// check runs Session.CheckCommand on command in the current cwd; Ctrl-C
// cancels it. A refusal is shown to the user and ok is false; the text of
// another error is never shown.
func (c *console) check(ctx context.Context, command string) (gateway.CommandCheck, bool) {
	lctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	c.in.disarm(func() { cancel(errInterrupted) })
	defer c.in.disarm(nil)
	chk, err := c.s.CheckCommand(lctx, command, c.cwd, c.home, c.out)
	switch {
	case err == nil:
		return chk, true
	case ctx.Err() != nil:
	case errors.Is(context.Cause(lctx), errInterrupted):
		c.write("^C\r\n")
		c.last = codeInterrupt
	default:
		c.fail(gateway.CommandErrorText(err))
	}
	return chk, false
}

// runOnTarget runs script on the target, its stdout on stdout and its stderr
// on the terminal, and returns its exit code. The first Ctrl-C sends INT to
// it, the second closes its channel (Ruling R4). A failed channel or a
// canceled ctx is code 1.
func (c *console) runOnTarget(ctx context.Context, script string, stdout io.Writer) int {
	ectx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	sig := make(chan string, 1)
	presses := 0 // the pump calls the handler with the input locked
	c.in.disarm(func() {
		if presses++; presses > 1 {
			cancel(errInterrupted)
			return
		}
		select {
		case sig <- "INT":
		default:
		}
	})
	defer c.in.disarm(nil)
	code, err := c.exec(ectx, script, stdout, c.t, sig)
	switch {
	case ctx.Err() != nil:
		return 1
	case errors.Is(context.Cause(ectx), errInterrupted):
		c.write("^C\r\n")
		return codeInterrupt
	case err != nil:
		c.say("Connection to " + c.s.Resource.Name + " failed")
		return 1
	}
	return code
}

// record adds a line that ran to the session result: the last line and the
// classes and paths of every line, sorted and unique.
func (c *console) record(line string, chk gateway.CommandCheck) {
	c.s.Command = line
	c.s.Classes = union(c.s.Classes, chk.Classes)
	c.s.Paths = union(c.s.Paths, chk.Paths)
	c.s.Paths = c.s.Paths[:min(len(c.s.Paths), maxPaths)]
}

func union(a, b []string) []string {
	out := slices.Concat(a, b)
	slices.Sort(out)
	return slices.Compact(out)
}

// marker is the recording marker of a line: "<line> [<classes>]".
func marker(line string, classes []string) string {
	return line + " [" + strings.Join(classes, ",") + "]"
}
