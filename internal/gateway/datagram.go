package gateway

import (
	"sync"

	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/sshdgram"
)

// channelDatagrams are the datagrams of a datagram@werbot.com channel, one
// sshdgram frame each. The connector is the only reader; Send takes a lock
// because a frame is two writes and flows answer concurrently.
type channelDatagrams struct {
	ch gossh.Channel
	mu sync.Mutex
}

// Recv reads one frame; one larger than buf is dropped with io.ErrShortBuffer.
func (d *channelDatagrams) Recv(buf []byte) (uint32, int, error) {
	return sshdgram.ReadFrame(d.ch, buf)
}

func (d *channelDatagrams) Send(flow uint32, p []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return sshdgram.WriteFrame(d.ch, flow, p)
}

func (d *channelDatagrams) Close() error { return d.ch.Close() }
