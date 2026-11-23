package connectcli

import (
	"context"
	"errors"
	"net"
	"time"
)

// Endpoint is where the gateway listens. Stage 15 adds a WebSocket URL.
// ponytail: Endpoint and Dialer are the plan's seam for stage 15; nothing sets
// mcpserver.Options.Dial yet. Cut them if stage 15 does not use them.
type Endpoint struct{ SSHAddress string }

// Dialer opens the transport connection to the gateway.
type Dialer func(ctx context.Context, ep Endpoint) (net.Conn, error)

var _ Dialer = Dial

// Dial connects over TCP to ep.SSHAddress, within 10 s.
func Dial(ctx context.Context, ep Endpoint) (net.Conn, error) {
	if ep.SSHAddress == "" {
		return nil, errors.New("gateway address is not set")
	}
	d := net.Dialer{Timeout: 10 * time.Second}
	return d.DialContext(ctx, "tcp", ep.SSHAddress)
}
