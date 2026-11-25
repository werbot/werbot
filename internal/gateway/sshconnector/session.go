package sshconnector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/gateway"
)

// command is what a terminal or an exec runs in a target session.
type command struct {
	line           string // "" is a shell
	term           string // "" is no PTY
	window         gateway.Window
	resize         <-chan gateway.Window
	stdin          io.Reader
	stdout, stderr io.Writer
	setExit        func(code int)
}

// terminalCommand merges stdout and stderr into the terminal.
func terminalCommand(t *gateway.Terminal) command {
	out := &lockedWriter{w: t}
	return command{line: t.Command, term: t.Term, window: t.Window, resize: t.Resize,
		stdin: t, stdout: out, stderr: out, setExit: t.SetExitStatus}
}

func execCommand(e *gateway.Exec) command {
	return command{line: e.Command, stdin: e.Stdin, stdout: e.Stdout, stderr: e.Stderr, setExit: e.SetExitStatus}
}

// run starts cmd in a new target session, relays its streams until it ends and
// reports the exit status. Every relay goroutine is guarded: a panic stops the
// session and gateway.Guard.Wait re-panics it on the calling goroutine.
func (cmd command) run(ctx context.Context, client *gossh.Client, rec gateway.RecordingSink, secret gateway.Secret) error {
	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh: open session: %w", err)
	}
	defer sess.Close()
	if cmd.term != "" {
		if err := sess.RequestPty(cmd.term, cmd.window.Rows, cmd.window.Cols, gossh.TerminalModes{}); err != nil {
			return fmt.Errorf("ssh: request pty: %w", err)
		}
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return err
	}
	if cmd.line == "" {
		err = sess.Shell()
	} else {
		err = sess.Start(cmd.line)
	}
	if err != nil {
		return fmt.Errorf("ssh: start: %w", err)
	}

	g := gateway.NewGuard(func() { _ = client.Close() }, secret)
	live := gateway.NewLiveSink(rec)
	defer live.End()
	ended := make(chan struct{})
	defer close(ended)
	g.Detach(func() error {
		_ = copyRec(stdin, cmd.stdin, live.Input) // a read or write error is the end of the input
		_ = stdin.Close()                         // EOF to the target
		return nil
	})
	if cmd.resize != nil {
		g.Detach(func() error { return relayResize(sess, cmd.resize, live, ended) })
	}
	g.Go(func() error { return copyRec(cmd.stdout, stdout, rec.Output) })
	g.Go(func() error { return copyRec(cmd.stderr, stderr, rec.Output) })
	if err := g.Wait(); err != nil {
		return err
	}
	return cmd.exitStatus(ctx, sess.Wait())
}

// relayResize sends window changes to the target until the session ends.
func relayResize(sess *gossh.Session, resize <-chan gateway.Window, live *gateway.LiveSink, ended <-chan struct{}) error {
	for {
		select {
		case w, ok := <-resize:
			if !ok {
				return nil
			}
			_ = sess.WindowChange(w.Rows, w.Cols) // fails only once the session is gone
			live.Resize(w.Cols, w.Rows)
		case <-ended:
			return nil
		}
	}
}

// exitStatus reports the result of Wait. After a cancel the session has no status.
func (cmd command) exitStatus(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	code, err := exitCode(err)
	if err != nil {
		return err
	}
	cmd.setExit(code)
	return nil
}

// exitCode is the exit code in the result of Wait; a missing status is 255.
func exitCode(err error) (int, error) {
	var exitErr *gossh.ExitError
	var missing *gossh.ExitMissingError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exitErr):
		return exitErr.ExitStatus(), nil
	case errors.As(err, &missing):
		return 255, nil
	}
	return 0, fmt.Errorf("ssh: wait: %w", err)
}

// lockedWriter serializes the stdout and stderr relays writing to one terminal.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
