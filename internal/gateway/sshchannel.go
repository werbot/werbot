package gateway

import (
	"io"

	gossh "golang.org/x/crypto/ssh"
)

// earlyChannel accepts a new SSH channel at most once: in Open, for a
// connector with PreSession, or after it.
type earlyChannel struct {
	nc   gossh.NewChannel
	ch   gossh.Channel // nil until accepted
	err  error
	done bool // Accept ran
}

// open accepts the channel on its first call and returns the same channel after.
func (e *earlyChannel) open() (gossh.Channel, error) {
	if !e.done {
		e.done = true
		var reqs <-chan *gossh.Request
		if e.ch, reqs, e.err = e.nc.Accept(); e.err == nil {
			go gossh.DiscardRequests(reqs)
		}
	}
	return e.ch, e.err
}

// accept is OpenRequest.Accept.
func (e *earlyChannel) accept() (io.ReadWriteCloser, error) {
	ch, err := e.open()
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// reject refuses the channel with text as Prohibited; a channel already
// accepted is closed instead (Ruling G4).
func (e *earlyChannel) reject(text string) {
	if !e.done {
		_ = e.nc.Reject(gossh.Prohibited, text)
		return
	}
	if e.ch != nil {
		_ = e.ch.Close()
	}
}

// attach hands ch to s by kind; a stream Open accepted already stays s.Stream.
func attach(s *Session, kind Kind, ch gossh.Channel) {
	switch {
	case kind == KindDatagram:
		s.Datagram = &channelDatagrams{ch: ch}
	case s.Stream == nil:
		s.Stream = ch
	}
}
