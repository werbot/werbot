package console

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
)

const startCommand = "cd && pwd -P"

// syncBuffer is the user's screen: everything the console writes to the terminal.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// spySink records what the console sends to the session recording.
type spySink struct {
	mu      sync.Mutex
	out, in bytes.Buffer
	markers []string
	resizes []gateway.Window
}

func (s *spySink) Output(p []byte) { s.mu.Lock(); s.out.Write(p); s.mu.Unlock() }
func (s *spySink) Input(p []byte)  { s.mu.Lock(); s.in.Write(p); s.mu.Unlock() }
func (s *spySink) Resize(cols, rows int) {
	s.mu.Lock()
	s.resizes = append(s.resizes, gateway.Window{Cols: cols, Rows: rows})
	s.mu.Unlock()
}
func (s *spySink) Marker(text string) {
	s.mu.Lock()
	s.markers = append(s.markers, text)
	s.mu.Unlock()
}
func (s *spySink) Close(context.Context) error {
	return nil
}

func (s *spySink) snapshot() (out, in string, markers []string, resizes []gateway.Window) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.String(), s.in.String(), slices.Clone(s.markers), slices.Clone(s.resizes)
}

type checkCall struct{ command, cwd, home string }

// targetFunc is the fake target: it answers one command.
type targetFunc func(ctx context.Context, command string, stdout io.Writer, sig <-chan string) (int, error)

// checkFunc is the fake checker of one command.
type checkFunc func(ctx context.Context, command string) (gateway.CommandCheck, error)

// harness runs Run in a synctest bubble against a fake target and checker.
type harness struct {
	t      *testing.T
	kb     *io.PipeWriter
	screen *syncBuffer
	rec    *spySink
	s      *gateway.Session
	resize chan gateway.Window
	cancel context.CancelFunc
	done   chan error

	mu     sync.Mutex
	checks []checkCall
	execs  []string
	exit   []int
	target targetFunc
	check  checkFunc
}

// home is a target whose login shell starts in home and whose cd answers the
// directory it was given.
func home(dir string) targetFunc {
	return func(_ context.Context, command string, stdout io.Writer, _ <-chan string) (int, error) {
		if command == startCommand {
			_, _ = io.WriteString(stdout, dir+"\n")
			return 0, nil
		}
		if d, ok := strings.CutSuffix(command, " && pwd -P"); ok {
			_, _ = io.WriteString(stdout, strings.Trim(strings.TrimPrefix(d, "cd -- "), "'")+"\n")
		}
		return 0, nil
	}
}

// answering is the target home("/home/u") with other answers for some commands.
func answering(answers map[string]targetFunc) targetFunc {
	base := home("/home/u")
	return func(ctx context.Context, command string, stdout io.Writer, sig <-chan string) (int, error) {
		if f, ok := answers[command]; ok {
			return f(ctx, command, stdout, sig)
		}
		return base(ctx, command, stdout, sig)
	}
}

// output is a target answer: text on stdout and an exit code.
func output(text string, code int) targetFunc {
	return func(_ context.Context, _ string, stdout io.Writer, _ <-chan string) (int, error) {
		_, _ = io.WriteString(stdout, text)
		return code, nil
	}
}

func allowRead(context.Context, string) (gateway.CommandCheck, error) {
	return gateway.CommandCheck{Classes: []string{"read"}}, nil
}

// newHarness starts Run with the target and checker; nil means home("/home/u")
// and an allow with classes [read].
func newHarness(t *testing.T, target targetFunc, check checkFunc) *harness {
	t.Helper()
	if target == nil {
		target = home("/home/u")
	}
	if check == nil {
		check = allowRead
	}
	h := &harness{t: t, screen: &syncBuffer{}, rec: &spySink{}, resize: make(chan gateway.Window, 1),
		done: make(chan error, 1), target: target, check: check}
	pr, pw := io.Pipe()
	h.kb = pw
	rw := struct {
		io.Reader
		io.Writer
	}{pr, h.screen}
	h.s = &gateway.Session{
		User:      gateway.User{Login: "alice"},
		Resource:  gateway.Resource{Name: "db"},
		Console:   true,
		Recording: h.rec,
		Terminal: &gateway.Terminal{ReadWriter: rw, Term: "xterm", Window: gateway.Window{Cols: 80, Rows: 24},
			Resize: h.resize, SetExitStatus: h.setExit},
	}
	h.s.CheckCommand = h.checkCommand
	t.Cleanup(func() { _ = pw.Close(); h.cancel() })
	return h
}

func (h *harness) run() *harness {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.done <- Run(ctx, h.s, h.s.Terminal.ReadWriter, h.exec) }()
	synctest.Wait()
	return h
}

func (h *harness) setExit(code int) {
	h.mu.Lock()
	h.exit = append(h.exit, code)
	h.mu.Unlock()
}

func (h *harness) checkCommand(ctx context.Context, command, cwd, home string, _ io.Writer) (gateway.CommandCheck, error) {
	h.mu.Lock()
	h.checks = append(h.checks, checkCall{command, cwd, home})
	h.mu.Unlock()
	return h.check(ctx, command)
}

func (h *harness) exec(ctx context.Context, command string, stdout, _ io.Writer, sig <-chan string) (int, error) {
	h.mu.Lock()
	h.execs = append(h.execs, command)
	h.mu.Unlock()
	return h.target(ctx, command, stdout, sig)
}

// typ sends keys and waits until the console is idle again.
func (h *harness) typ(keys ...string) {
	h.t.Helper()
	for _, k := range keys {
		_, err := io.WriteString(h.kb, k)
		require.NoError(h.t, err)
		synctest.Wait()
	}
}

// ended returns Run's result; Run must have returned.
func (h *harness) ended() error {
	h.t.Helper()
	synctest.Wait()
	select {
	case err := <-h.done:
		return err
	default:
		h.t.Fatal("Run is still running")
		return nil
	}
}

// running fails when Run has returned.
func (h *harness) running() {
	h.t.Helper()
	synctest.Wait()
	select {
	case err := <-h.done:
		h.t.Fatalf("Run returned %v", err)
	default:
	}
}

func (h *harness) calls() (checks []checkCall, execs []string, exit []int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.checks), slices.Clone(h.execs), slices.Clone(h.exit)
}

// commands lists the checked commands.
func (h *harness) commands() []string {
	checks, _, _ := h.calls()
	var out []string
	for _, c := range checks {
		out = append(out, c.command)
	}
	return out
}

// endsWith requires the screen to end with suffix.
func (h *harness) endsWith(suffix string) {
	h.t.Helper()
	require.True(h.t, strings.HasSuffix(h.screen.String(), suffix), "screen %q does not end with %q", h.screen.String(), suffix)
}
