package gateway

import "strings"

// TruncateUTF8 cuts s to at most n bytes without splitting a UTF-8 character
// and drops invalid bytes; a negative n is 0. Heartbeat strings go through it: the server bounds
// them in bytes and rejects the whole heartbeat for one string over its bound.
func TruncateUTF8(s string, n int) string {
	n = max(n, 0)
	s = strings.ToValidUTF8(s, "")
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
