//go:build windows

package cli

import (
	"errors"
	"io"
	"os"
)

// console is the console of the process: CONIN$ to read, CONOUT$ to write.
type console struct{ in, out *os.File }

func openTTY() (io.ReadWriteCloser, error) {
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		_ = in.Close()
		return nil, err
	}
	return console{in, out}, nil
}

func (c console) Read(p []byte) (int, error)  { return c.in.Read(p) }
func (c console) Write(p []byte) (int, error) { return c.out.Write(p) }

// Fd is the input handle, so a terminal answer is read without echo.
func (c console) Fd() uintptr { return c.in.Fd() }

func (c console) Close() error { return errors.Join(c.in.Close(), c.out.Close()) }
