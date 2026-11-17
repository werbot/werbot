package connectcli

import (
	"context"
	"net"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
)

// failingListener fails Accept with EMFILE fails times, then is closed.
type failingListener struct {
	net.Listener
	fails, calls int
}

func (l *failingListener) Accept() (net.Conn, error) {
	l.calls++
	if l.calls <= l.fails {
		return nil, syscall.EMFILE
	}
	return nil, net.ErrClosed
}

// An Accept error other than a closed listener is retried after a pause.
func TestAcceptRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ln := &failingListener{fails: 3}
		(&binding{ln: ln}).serveTCP(context.Background())
		assert.Equal(t, 4, ln.calls)
	})
}
