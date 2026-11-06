package telnet

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
		t.Fatalf("write: %v", err)
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

func newServerConn(t *testing.T, local, peer net.Conn) *Conn {
	t.Helper()

	type result struct {
		conn *Conn
		err  error
	}
	created := make(chan result, 1)
	go func() {
		c, err := NewServer(local)
		created <- result{conn: c, err: err}
	}()

	wantHandshake := []byte{
		IAC, WILL, optEcho,
		IAC, WILL, optSGA,
		IAC, DO, optNAWS,
		IAC, DO, optTTYPE,
	}
	if got := readExact(t, peer, len(wantHandshake)); !bytes.Equal(got, wantHandshake) {
		t.Fatalf("handshake = %v, want %v", got, wantHandshake)
	}

	res := <-created
	if res.err != nil {
		t.Fatalf("NewServer: %v", res.err)
	}
	return res.conn
}

func TestReadStripsIAC(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")

	go func() {
		writeAll(t, peer, []byte("ab"))
		writeAll(t, peer, []byte{IAC, WILL, optEcho})

		want := []byte{IAC, DO, optEcho}
		if got := readExact(t, peer, len(want)); !bytes.Equal(got, want) {
			t.Errorf("reply = %v, want %v", got, want)
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
		if got := readExact(t, peer, len(want)); !bytes.Equal(got, want) {
			t.Errorf("reply = %v, want %v", got, want)
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
		if got := readExact(t, peer, len(want)); !bytes.Equal(got, want) {
			t.Errorf("written = %v, want %v", got, want)
		}
	}()

	if _, err := c.Write([]byte{'x', 0xFF}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	<-done
}

func TestClientRefusesUnknownOption(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")
	startReader(t, c)

	go func() {
		writeAll(t, peer, []byte{IAC, DO, 99})
		writeAll(t, peer, []byte{IAC, WILL, 99})
	}()

	want := []byte{IAC, WONT, 99, IAC, DONT, 99}
	if got := readExact(t, peer, len(want)); !bytes.Equal(got, want) {
		t.Fatalf("reply = %v, want %v", got, want)
	}
}

func TestClientTTYPE(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")
	startReader(t, c)

	go func() {
		writeAll(t, peer, []byte{IAC, DO, optTTYPE})
	}()

	wantWill := []byte{IAC, WILL, optTTYPE}
	if got := readExact(t, peer, len(wantWill)); !bytes.Equal(got, wantWill) {
		t.Fatalf("reply = %v, want %v", got, wantWill)
	}

	go func() {
		writeAll(t, peer, []byte{IAC, SB, optTTYPE, ttypeSEND, IAC, SE})
	}()

	wantIS := []byte{IAC, SB, optTTYPE, ttypeIS, 'x', 't', 'e', 'r', 'm', IAC, SE}
	if got := readExact(t, peer, len(wantIS)); !bytes.Equal(got, wantIS) {
		t.Fatalf("reply = %v, want %v", got, wantIS)
	}
}

func TestSetWindowSizeBeforeNegotiation(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")
	startReader(t, c)

	if err := c.SetWindowSize(80, 24); err != nil {
		t.Fatalf("SetWindowSize: %v", err)
	}

	_ = peer.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := peer.Read(make([]byte, 1)); err == nil {
		t.Fatal("peer received data before NAWS negotiation")
	}

	_ = peer.SetReadDeadline(time.Now().Add(testDeadline))
	go func() {
		writeAll(t, peer, []byte{IAC, DO, optNAWS})
	}()

	want := []byte{IAC, WILL, optNAWS, IAC, SB, optNAWS, 0, 80, 0, 24, IAC, SE}
	if got := readExact(t, peer, len(want)); !bytes.Equal(got, want) {
		t.Fatalf("reply = %v, want %v", got, want)
	}
}

func TestSetWindowSizeEscapesIAC(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")
	startReader(t, c)

	go func() {
		writeAll(t, peer, []byte{IAC, DO, optNAWS})
	}()

	wantWill := []byte{IAC, WILL, optNAWS}
	if got := readExact(t, peer, len(wantWill)); !bytes.Equal(got, wantWill) {
		t.Fatalf("reply = %v, want %v", got, wantWill)
	}

	go func() {
		if err := c.SetWindowSize(255, 24); err != nil {
			t.Errorf("SetWindowSize: %v", err)
		}
	}()

	want := []byte{IAC, SB, optNAWS, 0, IAC, IAC, 0, 24, IAC, SE}
	if got := readExact(t, peer, len(want)); !bytes.Equal(got, want) {
		t.Fatalf("reply = %v, want %v", got, want)
	}
}

func TestServerHandshakeAndNAWS(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := newServerConn(t, local, peer)

	resized := make(chan [2]int, 1)
	c.OnResize = func(width, height int) {
		resized <- [2]int{width, height}
	}

	go func() {
		writeAll(t, peer, []byte{IAC, WILL, optNAWS})
		writeAll(t, peer, []byte{IAC, SB, optNAWS, 0, 120, 0, 40, IAC, SE})
		writeAll(t, peer, []byte("x"))
	}()

	if got := readUntil(t, c, "x"); got != "x" {
		t.Fatalf("Read() = %q, want %q", got, "x")
	}

	if width, height := c.Size(); width != 120 || height != 40 {
		t.Fatalf("Size() = %d,%d, want 120,40", width, height)
	}

	select {
	case got := <-resized:
		if got != [2]int{120, 40} {
			t.Fatalf("OnResize = %v, want 120,40", got)
		}
	case <-time.After(testDeadline):
		t.Fatal("OnResize was not called")
	}
}

func TestServerTTYPE(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := newServerConn(t, local, peer)

	readCh := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := c.Read(buf)
		if err != nil {
			readCh <- ""
			return
		}
		readCh <- string(buf[:n])
	}()

	go func() {
		writeAll(t, peer, []byte{IAC, WILL, optTTYPE})
	}()

	wantSend := []byte{IAC, SB, optTTYPE, ttypeSEND, IAC, SE}
	if got := readExact(t, peer, len(wantSend)); !bytes.Equal(got, wantSend) {
		t.Fatalf("reply = %v, want %v", got, wantSend)
	}

	go func() {
		writeAll(t, peer, []byte{IAC, SB, optTTYPE, ttypeIS, 'v', 't', '1', '0', '0', IAC, SE})
		writeAll(t, peer, []byte("x"))
	}()

	if got := <-readCh; got != "x" {
		t.Fatalf("Read() = %q, want %q", got, "x")
	}
	if got := c.TermType(); got != "vt100" {
		t.Fatalf("TermType() = %q, want %q", got, "vt100")
	}
}

func TestClientSGA(t *testing.T) {
	t.Parallel()

	local, peer := pipePair(t)
	c := NewClient(local, "xterm")
	startReader(t, c)

	go func() {
		writeAll(t, peer, []byte{IAC, DO, optSGA})
	}()

	wantWill := []byte{IAC, WILL, optSGA}
	if got := readExact(t, peer, len(wantWill)); !bytes.Equal(got, wantWill) {
		t.Fatalf("reply = %v, want %v", got, wantWill)
	}

	go func() {
		writeAll(t, peer, []byte{IAC, WILL, optSGA})
	}()

	wantDo := []byte{IAC, DO, optSGA}
	if got := readExact(t, peer, len(wantDo)); !bytes.Equal(got, wantDo) {
		t.Fatalf("reply = %v, want %v", got, wantDo)
	}
}
