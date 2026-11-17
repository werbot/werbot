package sshdgram

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var wire bytes.Buffer
	sizes := []int{0, 1, 8192, MaxPayload}
	for i, n := range sizes {
		if err := WriteFrame(&wire, uint32(i)+1, bytes.Repeat([]byte{byte(i + 'a')}, n)); err != nil {
			t.Fatalf("write %d bytes: %v", n, err)
		}
	}
	buf := make([]byte, MaxPayload)
	for i, n := range sizes {
		flow, got, err := ReadFrame(&wire, buf)
		if err != nil || flow != uint32(i)+1 || got != n {
			t.Fatalf("frame %d: flow %d, %d bytes, %v; want flow %d, %d bytes", i, flow, got, err, i+1, n)
		}
		if !bytes.Equal(buf[:got], bytes.Repeat([]byte{byte(i + 'a')}, n)) {
			t.Fatalf("frame %d: payload differs", i)
		}
	}
	if _, _, err := ReadFrame(&wire, buf); err != io.EOF {
		t.Fatalf("after the last frame: %v, want io.EOF", err)
	}
}

func TestFrameHeaderBigEndian(t *testing.T) {
	var wire bytes.Buffer
	if err := WriteFrame(&wire, 0x01020304, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if want := []byte{1, 2, 3, 4, 0, 2, 'h', 'i'}; !bytes.Equal(wire.Bytes(), want) {
		t.Fatalf("wire % x, want % x", wire.Bytes(), want)
	}
}

func TestWriteFrameTooLarge(t *testing.T) {
	var wire bytes.Buffer
	if err := WriteFrame(&wire, 1, make([]byte, MaxPayload+1)); err == nil {
		t.Fatal("a payload over MaxPayload must fail")
	}
	if wire.Len() != 0 {
		t.Fatalf("wrote %d bytes of a refused frame", wire.Len())
	}
}

// A frame larger than buf is read to its end and dropped; the next one is intact.
func TestReadFrameShortBuffer(t *testing.T) {
	var wire bytes.Buffer
	_ = WriteFrame(&wire, 7, []byte("0123456789"))
	_ = WriteFrame(&wire, 8, []byte("ok"))
	buf := make([]byte, 4)
	flow, n, err := ReadFrame(&wire, buf)
	if !errors.Is(err, io.ErrShortBuffer) || flow != 7 || n != 0 {
		t.Fatalf("flow %d, %d bytes, %v; want flow 7, 0 bytes, io.ErrShortBuffer", flow, n, err)
	}
	flow, n, err = ReadFrame(&wire, buf)
	if err != nil || flow != 8 || string(buf[:n]) != "ok" {
		t.Fatalf("next frame: flow %d, %q, %v", flow, buf[:n], err)
	}
}

// A frame cut anywhere after its first byte is io.ErrUnexpectedEOF, with or
// without room for its payload.
func TestReadFrameTruncated(t *testing.T) {
	var full bytes.Buffer
	_ = WriteFrame(&full, 1, []byte("payload"))
	for cut := 1; cut < full.Len(); cut++ {
		for _, size := range []int{2, MaxPayload} {
			_, _, err := ReadFrame(bytes.NewReader(full.Bytes()[:cut]), make([]byte, size))
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("cut at %d, buffer %d: %v, want io.ErrUnexpectedEOF", cut, size, err)
			}
		}
	}
}

// A header claiming the largest length reads no more than that from the wire.
func TestReadFrameBoundsLength(t *testing.T) {
	hdr := make([]byte, 6)
	binary.BigEndian.PutUint16(hdr[4:], 0xffff)
	r := io.MultiReader(bytes.NewReader(hdr), bytes.NewReader(make([]byte, 0x10000+10)))
	_, n, err := ReadFrame(r, make([]byte, 16))
	if !errors.Is(err, io.ErrShortBuffer) || n != 0 {
		t.Fatalf("%d bytes, %v; want io.ErrShortBuffer", n, err)
	}
	rest, _ := io.ReadAll(r)
	if len(rest) != 11 { // 0x10000+10 - 0xffff
		t.Fatalf("%d bytes left after the frame, want 11", len(rest))
	}
}

func TestResourceInfoJSON(t *testing.T) {
	b, err := json.Marshal(ResourceInfo{Type: "tcp", Kinds: []string{"stream"}, Ports: []Port{{Port: 5432}}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"type":"tcp","kinds":["stream"],"ports":[{"port":5432,"end_port":0}]}`; string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}
}
