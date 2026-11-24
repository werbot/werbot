package gateway

import "net"

// transportKey holds the transport of a connection (transportOf).
type transportKey struct{}

// transportOf is the transport c reports through Transport(), as a connection
// of the WebSocket front does; "tcp" for any other, a PROXY protocol one included.
func transportOf(c net.Conn) string {
	if t, ok := c.(interface{ Transport() string }); ok {
		return t.Transport()
	}
	return "tcp"
}
