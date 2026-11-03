package pty

import (
	"encoding/binary"
	"testing"
)

func TestParsePtyRequestValid(t *testing.T) {
	const term = "xterm"
	const termLen = 5
	buf := make([]byte, 0, 4+termLen+4+4+4+4)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(termLen))
	buf = append(buf, lenBuf...)
	buf = append(buf, term...)
	for _, v := range []uint32{80, 24, 0, 0} {
		binary.BigEndian.PutUint32(lenBuf, v)
		buf = append(buf, lenBuf...)
	}

	got, ok := ParsePtyRequest(buf)
	if !ok {
		t.Fatal("expected a valid pty request")
	}
	if got.EnvTerm != "xterm" || got.Width != 80 || got.Height != 24 {
		t.Fatalf("unexpected pty: %+v", got)
	}
}

func TestParsePtyRequestRejectsOversizedTermLength(t *testing.T) {
	// 0xFFFFFFFC + 4 overflows uint32. The parser must reject the frame
	// instead of panicking on a slice bound computed from client data.
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, 0xFFFFFFFC)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParsePtyRequest panicked: %v", r)
		}
	}()

	if _, ok := ParsePtyRequest(buf); ok {
		t.Fatal("expected an invalid pty request")
	}
}
