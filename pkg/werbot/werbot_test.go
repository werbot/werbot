package werbot

import (
	"errors"
	"fmt"
	"os/exec"
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
