//go:build !windows

package cli

import (
	"io"
	"os"
)

// openTTY opens the controlling terminal.
func openTTY() (io.ReadWriteCloser, error) { return os.OpenFile("/dev/tty", os.O_RDWR, 0) }
