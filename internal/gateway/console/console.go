// Package console is the command console: a shell under the console
// obligation where the gateway reads each line itself, checks it with
// Session.CheckCommand and runs it on the target as a separate command
// without a PTY, in the checked working directory.
package console

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/term"

	"github.com/werbot/werbot/internal/gateway"
)

// ExecFunc runs command on the target without a PTY and with stdin closed,
// writing its output to stdout and stderr. A name received on sig is sent to
// the command as a signal; a cancel of ctx closes its channel. It returns the
// exit code after its last write; an error is a failed channel, not a failed
// command.
type ExecFunc func(ctx context.Context, command string, stdout, stderr io.Writer, sig <-chan string) (exitCode int, err error)

// helpText is what help prints.
const helpText = `Werbot command console: each line runs as a separate command after a policy check.
Built-in commands: cd <dir>, pwd, help, exit.
Not supported: full-screen programs (vim, top, less without a pipe) and shell state between lines (export, functions, aliases).
Ctrl-C interrupts the running command, Ctrl-D exits.
`

var errNoChecker = errors.New("console: no command checker")

// console is one running console. Only Run's goroutine uses its fields,
// except t and size, which a window change also sets.
type console struct {
	s         *gateway.Session
	exec      ExecFunc
	in        *input
	out       io.Writer // the user's terminal, recorded
	home, cwd string
	last      int // exit code of the last line, shown in the prompt

	mu   sync.Mutex
	t    *term.Terminal
	size gateway.Window
}

// Run serves the console of s on rw (the session terminal) until the user
// exits, the input ends or ctx is canceled, running every line through exec.
// It fails without Session.CheckCommand, and with gateway.ErrConsoleUnavailable
// when the target's working directory cannot be read; the front prints that.
func Run(ctx context.Context, s *gateway.Session, rw io.ReadWriter, exec ExecFunc) error {
	if s.CheckCommand == nil {
		return errNoChecker
	}
	live := gateway.NewLiveSink(s.Recording)
	defer live.End() // input read after Run returns is not recorded (Ruling G33)
	in := newInput(rw, live)
	defer in.close()
	defer context.AfterFunc(ctx, in.close)()
	c := &console{s: s, exec: exec, in: in, out: recordedWriter{rw, s.Recording}, size: s.Terminal.Window}
	home, err := c.start(ctx)
	if err != nil || ctx.Err() != nil {
		return err
	}
	c.home, c.cwd = home, home
	c.newTerminal(nil)
	ended := make(chan struct{})
	var wg sync.WaitGroup
	defer wg.Wait()
	defer close(ended)
	wg.Go(func() { c.relayResize(s.Terminal.Resize, live, ended) })
	c.say("Werbot command console on " + s.Resource.Name + ". Type help for details.")
	c.loop(ctx)
	return nil
}

// start reads the home directory on the target; it is also the first cwd
// (Rulings G30, G31).
func (c *console) start(ctx context.Context) (string, error) {
	var out answer
	code, err := c.exec(ctx, "cd && pwd -P", &out, io.Discard, nil)
	switch dir, ok := out.dir(); {
	case ctx.Err() != nil:
		return "", nil
	case err != nil:
		return "", fmt.Errorf("%w: %w", gateway.ErrConsoleUnavailable, err)
	case code != 0 || !ok:
		return "", gateway.ErrConsoleUnavailable
	default:
		return dir, nil
	}
}

// loop reads and runs lines until the console ends.
func (c *console) loop(ctx context.Context) {
	for {
		line, ok := c.readLine(ctx)
		if !ok || c.handle(ctx, line) || ctx.Err() != nil {
			return
		}
	}
}

// readLine reads a line at the prompt; ok is false when the console ends.
// Ctrl-C drops the line and gives "".
func (c *console) readLine(ctx context.Context) (line string, ok bool) {
	c.t.SetPrompt(prompt(c.s.User.Login, c.s.Resource.Name, fold(c.cwd, c.home), c.last))
	c.in.arm()
	line, err := c.t.ReadLine()
	c.in.disarm(nil)
	switch {
	case ctx.Err() != nil:
		return "", false
	case err == nil:
		return line, true
	case errors.Is(err, io.EOF) && c.in.ctrlC:
		c.write("^C\r\n")
		c.newTerminal(c.t.History) // term keeps the dropped line
		return "", true
	case errors.Is(err, io.EOF): // Ctrl-D on an empty line, or the input closed
		c.s.Terminal.SetExitStatus(0)
	default:
		c.say("Console input error")
		c.s.Terminal.SetExitStatus(1)
	}
	return "", false
}

// handle runs one line; it reports whether the console ends.
func (c *console) handle(ctx context.Context, line string) (exit bool) {
	if !validLine(line) {
		c.fail("Command denied: control characters or line too long")
		return false
	}
	words := strings.Fields(line)
	switch {
	case len(words) == 0:
		return false
	case words[0] == "cd":
		c.cd(ctx, words[1:])
		return false
	case len(words) == 1:
		if exit, ok := c.builtin(words[0]); ok {
			return exit
		}
	}
	c.run(ctx, line)
	return false
}

// builtin runs exit, pwd or help: a builtin is one exact word (Ruling G32).
func (c *console) builtin(word string) (exit, ok bool) {
	switch word {
	case "exit":
		c.s.Terminal.SetExitStatus(0)
		return true, true
	case "pwd":
		c.say(c.cwd)
	case "help":
		c.write(strings.ReplaceAll(helpText, "\n", "\r\n"))
	default:
		return false, false
	}
	c.last = 0
	return false, true
}

// newTerminal starts a line editor at the current size with history (nil: a new one).
func (c *console) newTerminal(history term.History) {
	t := term.NewTerminal(struct {
		io.Reader
		io.Writer
	}{c.in, c.out}, "")
	if history != nil {
		t.History = history
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.size.Cols > 0 { // no PTY: 80 columns (Ruling G34)
		_ = t.SetSize(c.size.Cols, c.size.Rows)
	}
	c.t = t
}

// relayResize applies and records the window changes until ended.
func (c *console) relayResize(resize <-chan gateway.Window, live *gateway.LiveSink, ended <-chan struct{}) {
	for {
		select {
		case w, ok := <-resize:
			if !ok {
				return
			}
			c.mu.Lock()
			c.size = w
			_ = c.t.SetSize(w.Cols, w.Rows)
			c.mu.Unlock()
			live.Resize(w.Cols, w.Rows)
		case <-ended:
			return
		}
	}
}

// say writes a line of console text.
func (c *console) say(text string) { c.write(text + "\r\n") }

// fail writes a refusal; the next prompt shows code 1.
func (c *console) fail(text string) {
	c.say(text)
	c.last = 1
}

func (c *console) write(text string) { _, _ = io.WriteString(c.out, text) }

// recordedWriter writes to the user's terminal and records what it wrote.
type recordedWriter struct {
	w   io.Writer
	rec gateway.RecordingSink
}

func (r recordedWriter) Write(p []byte) (int, error) {
	n, err := r.w.Write(p)
	r.rec.Output(p[:n])
	return n, err
}

// fold shows cwd with home as "~"; a home of "/" is not folded.
func fold(cwd, home string) string {
	switch {
	case home == "/":
		return cwd
	case cwd == home:
		return "~"
	case strings.HasPrefix(cwd, home+"/"):
		return "~" + cwd[len(home):]
	}
	return cwd
}

// prompt is "<login>@<resource>:<cwd>$ ", after "[<code>] " when code is not 0.
func prompt(login, resource, cwd string, code int) string {
	p := login + "@" + resource + ":" + cwd + "$ "
	if code != 0 {
		p = "[" + strconv.Itoa(code) + "] " + p
	}
	return p
}
