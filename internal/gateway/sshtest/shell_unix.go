//go:build unix

package sshtest

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/gliderlabs/ssh"
)

// signals are the signal requests the target delivers.
var signals = map[ssh.Signal]syscall.Signal{
	ssh.SIGHUP: syscall.SIGHUP, ssh.SIGINT: syscall.SIGINT, ssh.SIGQUIT: syscall.SIGQUIT,
	ssh.SIGKILL: syscall.SIGKILL, ssh.SIGTERM: syscall.SIGTERM,
}

// Shell makes exec run the raw command with /bin/sh -c in home, with
// HOME=home, in its own process group; a signal request goes to that group
// and a command ended by a signal exits with 128 + its number. The shell and
// the PTY do not change.
func Shell(home string) Option {
	return func(tg *Target) { tg.exec = func(s ssh.Session) int { return tg.sh(home, s) } }
}

func (tg *Target) sh(home string, s ssh.Session) int {
	cmd := exec.Command("/bin/sh", "-c", s.RawCommand()) //nolint:gosec // a test target runs what the test sends
	cmd.Dir, cmd.Env = home, []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = s, s, s.Stderr()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = time.Second // a client that keeps stdin open does not hold Wait
	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(s.Stderr(), "sshtest: %v\n", err)
		return 127
	}
	sigs, done := make(chan ssh.Signal, 4), make(chan struct{})
	s.Signals(sigs)
	go tg.relaySignals(-cmd.Process.Pid, sigs, done)
	_ = cmd.Wait()
	s.Signals(nil) // later requests are buffered by gliderlabs; the relay drains until then
	close(done)
	if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return cmd.ProcessState.ExitCode()
}

// relaySignals sends the signal requests to the process group pgid until done.
func (tg *Target) relaySignals(pgid int, sigs <-chan ssh.Signal, done <-chan struct{}) {
	for {
		select {
		case sig := <-sigs:
			tg.event("signal:" + string(sig))
			if n, ok := signals[sig]; ok {
				_ = syscall.Kill(pgid, n)
			}
		case <-done:
			return
		}
	}
}
