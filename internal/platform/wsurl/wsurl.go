// Package wsurl checks the WebSocket URL of a gateway pool: the server's
// ws_url and the client's --gateway.
package wsurl

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// Valid reports whether s is a ws:// or wss:// URL (scheme in any case) of
// at most 2048 bytes with a host, a port of 1 to 65535 if any, and no
// userinfo, query, fragment, '%' or whitespace.
func Valid(s string) bool {
	u, err := url.Parse(s)
	if err != nil || len(s) > 2048 || (u.Scheme != "ws" && u.Scheme != "wss") || u.Hostname() == "" || u.User != nil ||
		strings.ContainsFunc(s, badRune) {
		return false
	}
	return validPort(u.Port())
}

// badRune is a character a URL never holds: query, fragment, escape, space or control.
func badRune(r rune) bool {
	return r == '?' || r == '#' || r == '%' || unicode.IsSpace(r) || unicode.IsControl(r)
}

// validPort: no port, or 1 to 65535.
func validPort(p string) bool {
	n, err := strconv.Atoi(p)
	return p == "" || (err == nil && n >= 1 && n <= 65535)
}
