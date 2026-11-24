package connectcli

import (
	"net"
	"sync"
	"testing"

	"github.com/gliderlabs/ssh"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/sshdgram"
)

// mfaGateway is a gateway front that wants only the MFA round: it refuses every
// key, asks prompt with keyboard-interactive and lets in whoever answers want.
type mfaGateway struct {
	*fakeGateway
	mu      sync.Mutex
	answers []string // every answer received
}

func startMFAGateway(t *testing.T, info *sshdgram.ResourceInfo, prompt, want string) *mfaGateway {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	g := &mfaGateway{fakeGateway: &fakeGateway{addr: ln.Addr().String(), hostKey: newKey(t), info: info}}
	srv := &ssh.Server{
		KeyboardInteractiveHandler: func(_ ssh.Context, ch gossh.KeyboardInteractiveChallenge) bool {
			ans, err := ch("", "", []string{prompt}, []bool{false})
			if err != nil || len(ans) != 1 {
				return false
			}
			g.mu.Lock()
			g.answers = append(g.answers, ans[0])
			g.mu.Unlock()
			return ans[0] == want
		},
		ChannelHandlers: map[string]ssh.ChannelHandler{"direct-tcpip": g.stream},
		RequestHandlers: map[string]ssh.RequestHandler{sshdgram.ResourceRequest: g.resource},
	}
	srv.AddHostKey(g.hostKey)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return g
}

func (g *mfaGateway) received() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.answers...)
}
