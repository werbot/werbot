package wsurl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValid(t *testing.T) {
	for _, s := range []string{"wss://gw.example.com/v1/ssh", "ws://gw:8088", "WSS://gw", "wss://gw:1", "wss://gw:65535/edge", "wss://[::1]:443/v1/ssh"} {
		assert.True(t, Valid(s), s)
	}
	for _, s := range []string{"", "ws://", "http://gw", "wss:///v1/ssh", "gw.example.com", "wss://u:p@gw", "wss://gw/?a=1", "wss://gw/?",
		"wss://gw/#f", "wss://gw/a%20b", "wss://gw/a b", "wss://gw:0", "wss://gw:70000", "wss://gw:x", "wss://gw/" + strings.Repeat("a", 2048)} {
		assert.False(t, Valid(s), s)
	}
}
