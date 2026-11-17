package telnetconnector

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

const testDeadline = 2 * time.Second

func pipePair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	left, right := net.Pipe()
	t.Cleanup(func() {
		_ = left.Close()
		_ = right.Close()
	})
	_ = left.SetDeadline(time.Now().Add(testDeadline))
	_ = right.SetDeadline(time.Now().Add(testDeadline))
	return left, right
}

func readExact(t *testing.T, c net.Conn, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatalf("read %d bytes: %v", n, err)
	}
	return buf
}

func writeAll(t *testing.T, c net.Conn, data []byte) {
	t.Helper()
	if _, err := c.Write(data); err != nil {
		t.Errorf("write: %v", err)
	}
}

func readUntil(t *testing.T, c *Conn, want string) string {
	t.Helper()
	var got bytes.Buffer
	buf := make([]byte, 64)
	for !bytes.Contains(got.Bytes(), []byte(want)) {
		n, err := c.Read(buf)
		if err != nil {
			t.Fatalf("read: %v (got %q)", err, got.String())
		}
		got.Write(buf[:n])
	}
	return got.String()
}

func startReader(t *testing.T, c *Conn) {
	t.Helper()
	go func() {
		buf := make([]byte, 256)
		for {
			if _, err := c.Read(buf); err != nil {
				return
			}
		}
	}()
}

func TestReadStripsIAC(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")

	go func() {
		writeAll(t, peer, []byte("ab"))
		writeAll(t, peer, []byte{IAC, WILL, optEcho})

		want := []byte{IAC, DO, optEcho}
		buf := make([]byte, len(want))
		if _, err := io.ReadFull(peer, buf); err != nil || !bytes.Equal(buf, want) {
			t.Errorf("reply = %v (%v), want %v", buf, err, want)
			return
		}
		writeAll(t, peer, []byte("c"))
	}()

	if got := readUntil(t, c, "abc"); got != "abc" {
		t.Fatalf("Read() = %q, want %q", got, "abc")
	}
}

func TestReadEscapedIAC(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")

	go func() {
		writeAll(t, peer, []byte{IAC, IAC})
	}()

	buf := make([]byte, 1)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if want := []byte{0xFF}; !bytes.Equal(buf, want) {
		t.Fatalf("Read() = %v, want %v", buf, want)
	}
}

func TestReadCRNUL(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")

	go func() {
		writeAll(t, peer, []byte("a\r\x00b"))
	}()

	if got := readUntil(t, c, "b"); got != "a\rb" {
		t.Fatalf("Read() = %q, want %q", got, "a\rb")
	}
}

func TestReadSplitIACAcrossWrites(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")
	if err := c.SetWindowSize(80, 24); err != nil {
		t.Fatalf("SetWindowSize: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		writeAll(t, peer, []byte{IAC})
		writeAll(t, peer, []byte{DO, optNAWS})

		// WILL NAWS followed by the current window size.
		want := []byte{IAC, WILL, optNAWS, IAC, SB, optNAWS, 0, 80, 0, 24, IAC, SE}
		buf := make([]byte, len(want))
		if _, err := io.ReadFull(peer, buf); err != nil || !bytes.Equal(buf, want) {
			t.Errorf("reply = %v (%v), want %v", buf, err, want)
			return
		}
		writeAll(t, peer, []byte("ok"))
	}()

	if got := readUntil(t, c, "ok"); got != "ok" {
		t.Fatalf("Read() = %q, want %q", got, "ok")
	}
	<-done
}

func TestWriteEscapesIAC(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")

	done := make(chan struct{})
	go func() {
		defer close(done)
		want := []byte{'x', IAC, IAC}
		buf := make([]byte, len(want))
		if _, err := io.ReadFull(peer, buf); err != nil || !bytes.Equal(buf, want) {
			t.Errorf("written = %v (%v), want %v", buf, err, want)
		}
	}()

	if _, err := c.Write([]byte{'x', 0xFF}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	<-done
}
