// Package mask hides credentials in a stream of command output: the literal
// values of known secrets and, optionally, text of a known credential shape.
// A secret split across writes is held until it is complete, so no write
// carries part of it.
package mask

import (
	"bytes"
	"io"
	"slices"
)

// minSecretLen is the shortest secret that is masked: a shorter one would
// mask ordinary text.
const minSecretLen = 4

// ponytail: an unfinished line is held up to 4 KiB, a longer one is checked
// as is; a pattern match split at that edge is not masked.
const patternWindow = 4096

// marker replaces each masked value.
var marker = []byte("[masked]")

// secret is a value with its KMP failure table.
type secret struct {
	v    []byte
	fail []int // fail[i]: length of the longest proper prefix of v[:i+1] that is also its suffix
}

func newSecret(v []byte) secret {
	fail := make([]int, len(v))
	for i, k := 1, 0; i < len(v); i++ {
		for k > 0 && v[i] != v[k] {
			k = fail[k-1]
		}
		if v[i] == v[k] {
			k++
		}
		fail[i] = k
	}
	return secret{v: v, fail: fail}
}

// prefixAtEnd is the length of the longest suffix of b that is a proper
// prefix of the secret.
func (s secret) prefixAtEnd(b []byte) int {
	k := 0
	for _, c := range b[max(0, len(b)-len(s.v)+1):] { // fewer bytes than the secret: k never reaches len(s.v)
		for k > 0 && s.v[k] != c {
			k = s.fail[k-1]
		}
		if s.v[k] == c {
			k++
		}
	}
	return k
}

type writer struct {
	w        io.Writer
	secrets  []secret // longest first
	patterns bool
	inKey    bool   // inside a private key block
	buf      []byte // held: a secret prefix, with patterns also an unfinished line
}

// NewWriter masks the secrets (at least minSecretLen bytes each) in what is
// written to w; with patterns it also masks credential-shaped text in whole
// lines. Close writes the held tail; an error of w is returned from Write.
func NewWriter(w io.Writer, secrets [][]byte, patterns bool) io.WriteCloser {
	m := &writer{w: w, patterns: patterns}
	for _, s := range secrets {
		if len(s) >= minSecretLen {
			m.secrets = append(m.secrets, newSecret(s))
		}
	}
	slices.SortFunc(m.secrets, func(a, b secret) int { return len(b.v) - len(a.v) })
	return m
}

// Bytes is p with the secrets and, with patterns, credential-shaped text
// masked as one whole write (a database cell): nothing is held back.
func Bytes(p []byte, secrets [][]byte, patterns bool) []byte {
	var out bytes.Buffer
	w := NewWriter(&out, secrets, patterns)
	_, _ = w.Write(p) // a bytes.Buffer never fails
	_ = w.Close()
	return out.Bytes()
}

func (m *writer) Write(p []byte) (int, error) {
	m.buf = m.literal(append(m.buf, p...))
	cut := len(m.buf) - m.held()
	if m.patterns {
		cut = lineCut(m.buf, cut)
	}
	if err := m.emit(m.buf[:cut]); err != nil {
		return 0, err
	}
	m.buf = m.buf[:copy(m.buf, m.buf[cut:])]
	return len(p), nil
}

func (m *writer) Close() error {
	err := m.emit(m.buf)
	m.buf = nil
	return err
}

// literal replaces every secret in b with the marker, longest first.
func (m *writer) literal(b []byte) []byte {
	for _, s := range m.secrets {
		if bytes.Contains(b, s.v) {
			b = bytes.ReplaceAll(b, s.v, marker)
		}
	}
	return b
}

// held is the length of the longest suffix of buf that may start a secret.
func (m *writer) held() int {
	h := 0
	for _, s := range m.secrets {
		h = max(h, s.prefixAtEnd(m.buf))
	}
	return h
}

// lineCut moves cut back to the end of the last whole line in b, unless the
// unfinished line is longer than patternWindow.
func lineCut(b []byte, cut int) int {
	i := bytes.LastIndexByte(b[:cut], '\n') + 1
	if len(b)-i <= patternWindow {
		return i
	}
	return cut
}

func (m *writer) emit(b []byte) error {
	if m.patterns {
		b = m.pattern(b)
	}
	if len(b) == 0 {
		return nil
	}
	_, err := m.w.Write(b)
	return err
}
