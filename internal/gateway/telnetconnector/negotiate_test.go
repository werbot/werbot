package telnetconnector

import (
	"bytes"
	"net"
	"testing"
	"testing/synctest"
	"time"
)

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

// Before the target agrees to NAWS the size is only stored; the bubble's clock
// makes the 100 ms quiet check exact.
func TestSetWindowSizeBeforeNegotiation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		_ = c.Close()
	})
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

// Negotiated closes once the client answered both NAWS and TTYPE, by value or by WONT.
func TestServerNegotiated(t *testing.T) {
	t.Parallel()

	for name, answer := range map[string][]byte{
		"values": {IAC, SB, optNAWS, 0, 132, 0, 40, IAC, SE, IAC, SB, optTTYPE, ttypeIS, 'v', 't', IAC, SE},
		"wont":   {IAC, WONT, optNAWS, IAC, WONT, optTTYPE},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			local, peer := pipePair(t)
			c := newServerConn(t, local, peer)
			startReader(t, c)
			select {
			case <-c.Negotiated():
				t.Fatal("negotiated before the client answered")
			default:
			}
			writeAll(t, peer, answer[:len(answer)/2])
			select {
			case <-c.Negotiated():
				t.Fatal("negotiated after half the answer")
			default:
			}
			writeAll(t, peer, answer[len(answer)/2:])
			select {
			case <-c.Negotiated():
			case <-time.After(testDeadline):
				t.Fatal("Negotiated did not close")
			}
		})
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
