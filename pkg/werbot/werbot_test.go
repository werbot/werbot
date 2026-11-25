package werbot

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codeErr int

func (codeErr) Error() string   { return "code" }
func (c codeErr) ExitCode() int { return int(c) }

func TestExitCode(t *testing.T) {
	code, report := exitCode(nil)
	assert.Equal(t, 0, code)
	assert.False(t, report)

	code, report = exitCode(codeErr(3))
	assert.Equal(t, 3, code)
	assert.False(t, report)

	code, report = exitCode(errors.New("boom"))
	assert.Equal(t, 1, code)
	assert.True(t, report)

	var ee *exec.ExitError
	require.ErrorAs(t, exec.Command("sh", "-c", "exit 2").Run(), &ee)
	code, report = exitCode(fmt.Errorf("ssh: %w", ee))
	assert.Equal(t, 2, code)
	assert.False(t, report)
}

// A closed terminal (SIGHUP) stops werbot like Ctrl+C, so --vnet removes its
// OS settings; a process started with SIGHUP ignored (nohup) keeps ignoring it.
func TestStopSignals(t *testing.T) {
	if signal.Ignored(syscall.SIGHUP) {
		t.Skip("the test runs with SIGHUP ignored")
	}
	assert.Equal(t, []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}, stopSignals())
	signal.Ignore(syscall.SIGHUP)
	t.Cleanup(func() { signal.Reset(syscall.SIGHUP) })
	assert.Equal(t, []os.Signal{syscall.SIGINT, syscall.SIGTERM}, stopSignals())
}
