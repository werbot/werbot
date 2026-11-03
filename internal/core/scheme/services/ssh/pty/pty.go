package pty

import "encoding/binary"

// Pty is ...
type Pty struct {
	Width  int
	Height int
	// Title   string
	EnvTerm string
	// EnvShell string
}

// ParsePtyRequest is ...
func ParsePtyRequest(s []byte) (pty Pty, ok bool) {
	term, s, ok := parseString(s)
	if !ok {
		return
	}
	width32, s, ok := parseUint32(s)
	if width32 < 1 {
		ok = false
	}
	if !ok {
		return
	}
	height32, _, ok := parseUint32(s)
	if height32 < 1 {
		ok = false
	}
	if !ok {
		return
	}

	pty = Pty{
		EnvTerm: term,
		Width:   int(width32),
		Height:  int(height32),
	}
	return
}

func parseString(in []byte) (out string, rest []byte, ok bool) {
	if len(in) < 4 {
		return
	}
	length := binary.BigEndian.Uint32(in)
	if uint64(length) > uint64(len(in)-4) { //nolint:gosec // G115: len(in) >= 4, so the subtraction is non-negative
		return
	}
	end := 4 + int(length)
	out = string(in[4:end])
	rest = in[end:]
	ok = true
	return
}

func parseUint32(in []byte) (uint32, []byte, bool) {
	if len(in) < 4 {
		return 0, nil, false
	}
	return binary.BigEndian.Uint32(in), in[4:], true
}
