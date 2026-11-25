package sshconnector

import (
	"context"
	"fmt"
	"io"

	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/console"
)

var _ gateway.ConsoleServer = Connector{}

// ServeConsole connects to the target and serves the command console of s on
// its terminal until it ends; every line runs in its own exec channel. A
// cancel of ctx closes the target connection and is not an error.
func (c Connector) ServeConsole(ctx context.Context, s *gateway.Session) error {
	client, closeClient, err := c.dial(ctx, s)
	if err == nil {
		defer closeClient()
		err = console.Run(ctx, s, s.Terminal, execOn(client))
	}
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// execOn runs each command in a new session channel on client, one at a time:
// no PTY, stdin at EOF. A name on sig goes to the command as a signal; a
// cancel of ctx closes the channel. It returns after the last write to stdout
// and stderr. A command ended by a signal exits with 128 + its number
// (x/crypto/ssh maps an exit-signal); a missing exit status is 255.
func execOn(client *gossh.Client) console.ExecFunc {
	return func(ctx context.Context, command string, stdout, stderr io.Writer, sig <-chan string) (int, error) {
		sess, err := client.NewSession()
		if err != nil {
			return 0, fmt.Errorf("ssh: open session: %w", err)
		}
		defer sess.Close()
		sess.Stdout, sess.Stderr = stdout, stderr // Stdin nil: EOF at once
		if err := sess.Start(command); err != nil {
			return 0, fmt.Errorf("ssh: start: %w", err)
		}
		defer context.AfterFunc(ctx, func() { _ = sess.Close() })()
		done := make(chan struct{})
		defer close(done)
		go relaySignals(sess, sig, done)
		return exitCode(sess.Wait()) // Wait returns after the stdout and stderr copies
	}
}

// relaySignals sends the names received on sig to sess until done.
func relaySignals(sess *gossh.Session, sig <-chan string, done <-chan struct{}) {
	for {
		select {
		case name, ok := <-sig:
			if !ok {
				return
			}
			_ = sess.Signal(gossh.Signal(name)) // fails only once the channel is gone
		case <-done:
			return
		}
	}
}
