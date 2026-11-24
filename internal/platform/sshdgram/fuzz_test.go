package sshdgram

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

// FuzzReadFrame: raw bytes never panic and never read past a frame; a frame
// WriteFrame wrote reads back unchanged, or, over the buffer, is dropped whole
// with io.ErrShortBuffer so the next frame stays readable.
func FuzzReadFrame(f *testing.F) {
	for _, n := range []int{0, 1, 8192, MaxPayload} {
		var b bytes.Buffer
		if err := WriteFrame(&b, 7, bytes.Repeat([]byte{'x'}, n)); err != nil {
			f.Fatal(err)
		}
		f.Add(b.Bytes(), uint32(7), 8192)
	}
	whole := []byte{0, 0, 0, 1, 0, 5, 'h', 'e', 'l', 'l', 'o'}
	for _, cut := range []int{2, 5, 6} {
		f.Add(whole[:cut], uint32(1), 4)
	}
	f.Add(append(append([]byte{}, whole...), whole...), uint32(1), 2) // two frames over the buffer
	f.Add([]byte{0, 0, 0, 1, 0xff, 0xff}, uint32(1), 0)               // length 65535, no payload
	f.Add([]byte("\xff\xff\xff\xff\xff\xff garbage"), uint32(0), 2)
	f.Fuzz(func(t *testing.T, data []byte, flow uint32, size int) {
		if len(data) > 1<<16 || size < 0 || size > 1<<16 {
			t.Skip()
		}
		readRaw(t, data, size)
		roundTrip(t, data[:min(len(data), MaxPayload)], flow, size)
	})
}

// readRaw reads data as frames until the first error. A frame that reads or
// is dropped as too long consumes exactly its header and declared length.
func readRaw(t *testing.T, data []byte, size int) {
	t.Helper()
	r, buf := bytes.NewReader(data), make([]byte, size)
	for {
		before := r.Len()
		_, n, err := ReadFrame(r, buf)
		if err != nil && !errors.Is(err, io.ErrShortBuffer) {
			return
		}
		want := headerLen + int(binary.BigEndian.Uint16(data[len(data)-before+4:]))
		if used := before - r.Len(); used != want {
			t.Fatalf("ReadFrame (err %v) read %d bytes, the frame is %d", err, used, want)
		}
		if !validLen(n, err, len(buf)) {
			t.Fatalf("ReadFrame n = %d, err %v, with a %d byte buffer", n, err, len(buf))
		}
	}
}

// validLen: a read frame fits the buffer, a dropped frame has no length.
func validLen(n int, err error, size int) bool {
	if err != nil {
		return n == 0
	}
	return n >= 0 && n <= size && n <= MaxPayload
}

// roundTrip writes p and a marker frame, then reads both back.
func roundTrip(t *testing.T, p []byte, flow uint32, size int) {
	t.Helper()
	var wire bytes.Buffer
	if err := WriteFrame(&wire, flow, p); err != nil {
		t.Fatalf("WriteFrame(%d bytes) = %v", len(p), err)
	}
	if err := WriteFrame(&wire, flow+1, []byte("next")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, size)
	got, n, err := ReadFrame(&wire, buf)
	if got != flow {
		t.Fatalf("ReadFrame flow = %d, want %d", got, flow)
	}
	if len(p) > size {
		if !errors.Is(err, io.ErrShortBuffer) {
			t.Fatalf("ReadFrame(%d bytes, buffer %d) = %v, want io.ErrShortBuffer", len(p), size, err)
		}
	} else if err != nil || n != len(p) || !bytes.Equal(buf[:n], p) {
		t.Fatalf("ReadFrame(%d bytes, buffer %d) = %d bytes, %v; want the payload back", len(p), size, n, err)
	}
	checkMarker(t, &wire, flow+1)
}

// checkMarker reads the frame after the dropped or read one.
func checkMarker(t *testing.T, r io.Reader, flow uint32) {
	t.Helper()
	next := make([]byte, 8)
	if got, n, err := ReadFrame(r, next); err != nil || got != flow || string(next[:n]) != "next" {
		t.Fatalf("next frame = flow %d %q, %v; want the marker frame", got, next[:n], err)
	}
}
