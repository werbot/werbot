package mask

import (
	"bytes"
	"math"
	"regexp"
)

var (
	// tokenRe matches credentials of a known shape: AWS access key ids,
	// GitHub tokens and JWTs.
	tokenRe = regexp.MustCompile(`(?:AKIA|ASIA)[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{82}|` +
		`eyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}`)
	// assignRe matches a value assigned to a credential-like key on one line;
	// group 1 is the value.
	assignRe = regexp.MustCompile(`(?i)(?:password|passwd|secret|token|api[_-]?key)\S*?[ \t]*[:=][ \t]*(\S+)`)

	// tokenHints and assignHints hold a literal of every match of tokenRe and
	// of lower-cased assignRe: without one the slow regexp scan is skipped.
	tokenHints  = []string{"AKIA", "ASIA", "gh", "github_pat_", "eyJ"}
	assignHints = []string{"passw", "secret", "token", "api"}

	pemBegin = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	pemEnd   = regexp.MustCompile(`-----END [A-Z ]*PRIVATE KEY-----`)
)

// An assigned value is masked from this length and Shannon entropy
// (bits per byte) up: shorter or more uniform values are rarely credentials.
const (
	minValueLen = 8
	minEntropy  = 3.0
)

// pattern masks credential-shaped text in b, which holds whole lines unless
// a line outgrew patternWindow.
func (m *writer) pattern(b []byte) []byte {
	b = m.pem(b)
	if hinted(b, tokenHints) {
		b = tokenRe.ReplaceAll(b, marker)
	}
	if hinted(bytes.ToLower(b), assignHints) {
		b = maskAssigned(b)
	}
	return b
}

func hinted(b []byte, hints []string) bool {
	for _, h := range hints {
		if bytes.Contains(b, []byte(h)) {
			return true
		}
	}
	return false
}

// pem turns each private key block into one marker; a block may span writes.
func (m *writer) pem(b []byte) []byte {
	var out []byte
	for {
		if m.inKey {
			loc := pemEnd.FindIndex(b)
			if loc == nil {
				return out
			}
			m.inKey, b = false, b[loc[1]:]
		}
		loc := pemBegin.FindIndex(b)
		if loc == nil {
			return append(out, b...)
		}
		out = append(append(out, b[:loc[0]]...), marker...)
		m.inKey, b = true, b[loc[1]:]
	}
}

// maskAssigned masks the assigned values of assignRe that look random enough.
func maskAssigned(b []byte) []byte {
	var out []byte
	last := 0
	for _, loc := range assignRe.FindAllSubmatchIndex(b, -1) {
		if v := b[loc[2]:loc[3]]; len(v) < minValueLen || entropy(v) < minEntropy {
			continue
		}
		out = append(append(out, b[last:loc[2]]...), marker...)
		last = loc[3]
	}
	if last == 0 {
		return b
	}
	return append(out, b[last:]...)
}

// entropy is the Shannon entropy of b in bits per byte.
func entropy(b []byte) float64 {
	var n [256]int
	for _, c := range b {
		n[c]++
	}
	h := 0.0
	for _, c := range n {
		if c > 0 {
			p := float64(c) / float64(len(b))
			h -= p * math.Log2(p)
		}
	}
	return h
}
