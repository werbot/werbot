package gateway

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// Typing in the resource menu keeps the connection; once the input stops, the
// connection idles out.
func TestIdleTimeoutMenuInput(t *testing.T) {
	const idle = 300 * time.Millisecond
	e := newSSHEnv(t, Config{IdleTimeout: idle})
	c := e.client(t, "alice")
	s, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	in, err := s.StdinPipe()
	require.NoError(t, err)
	s.Stdout = io.Discard
	require.NoError(t, s.Shell())
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()

	for range 12 { // 1.2 s of typing, four times the limit
		time.Sleep(idle / 3)
		select {
		case <-done:
			t.Fatal("the connection was closed while the user typed in the menu")
		default:
		}
		_, err := in.Write([]byte("1"))
		require.NoError(t, err)
	}
	select {
	case <-done:
	case <-time.After(5 * idle):
		t.Fatal("the menu without input is still open")
	}
}
