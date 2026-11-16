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
// session and guard.done re-panics it on the calling goroutine.
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

	g := &guard{stop: func() { _ = client.Close() }, secret: secret}
	live := &liveSink{rec: rec}
	defer live.end()
	ended := make(chan struct{})
	defer close(ended)
	g.detach(func() error {
		_ = copyRec(stdin, cmd.stdin, live.input) // a read or write error is the end of the input
		_ = stdin.Close()                         // EOF to the target
		return nil
	})
	if cmd.resize != nil {
		g.detach(func() error { return relayResize(sess, cmd.resize, live, ended) })
	}
	g.spawn(func() error { return copyRec(cmd.stdout, stdout, rec.Output) })
	g.spawn(func() error { return copyRec(cmd.stderr, stderr, rec.Output) })
	if err := g.done(); err != nil {
		return err
	}
	return cmd.exitStatus(ctx, sess.Wait())
}

// relayResize sends window changes to the target until the session ends.
func relayResize(sess *gossh.Session, resize <-chan gateway.Window, live *liveSink, ended <-chan struct{}) error {
	for {
		select {
		case w, ok := <-resize:
			if !ok {
				return nil
			}
			_ = sess.WindowChange(w.Rows, w.Cols) // fails only once the session is gone
			live.resize(w.Cols, w.Rows)
		case <-ended:
			return nil
		}
	}
}

// exitStatus reports the result of Wait; a missing status is 255. After a
// cancel the session has no status.
func (cmd command) exitStatus(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	var exitErr *gossh.ExitError
	var missing *gossh.ExitMissingError
	switch {
	case err == nil:
		cmd.setExit(0)
	case errors.As(err, &exitErr):
		cmd.setExit(exitErr.ExitStatus())
	case errors.As(err, &missing):
		cmd.setExit(255)
	default:
		return fmt.Errorf("ssh: wait: %w", err)
	}
	return nil
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

// liveSink passes the detached relays' recorder calls only until run returns:
// the input relay may still read from the front after that, when the pipeline
// already closes the recording.
type liveSink struct {
	mu    sync.Mutex
	ended bool
	rec   gateway.RecordingSink
}

func (l *liveSink) input(p []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.ended {
		l.rec.Input(p)
	}
}

func (l *liveSink) resize(cols, rows int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.ended {
		l.rec.Resize(cols, rows)
	}
}

func (l *liveSink) end() {
	l.mu.Lock()
	l.ended = true
	l.mu.Unlock()
}
