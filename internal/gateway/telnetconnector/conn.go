package telnetconnector

import (
	"io"
	"net"
	"sync"
	"time"
)

// Telnet protocol commands and options used by the connector.
const (
	IAC  = 255
	DONT = 254
	DO   = 253
	WONT = 252
	WILL = 251
	SB   = 250
	SE   = 240

	optEcho  = 1
	optSGA   = 3
	optTTYPE = 24
	optNAWS  = 31

	ttypeIS   = 0
	ttypeSEND = 1

	maxSubnegotiation = 256
)

type role int

const (
	roleClient role = iota
	roleServer
)

type readState int

const (
	stateData readState = iota
	stateIAC
	stateVerb
	stateSB
	stateSBIAC
)

// Conn wraps a connection and hides telnet negotiation from its readers.
// Read returns only application data; Write escapes IAC.
type Conn struct {
	rw   io.ReadWriter
	role role

	mu         sync.Mutex
	termType   string
	width      int
	height     int
	nawsAgreed bool
	willSent   map[byte]bool
	doSent     map[byte]bool
	wontSent   map[byte]bool
	dontSent   map[byte]bool
	sendTTYPE  bool
	writeErr   error

	// server side: the client answered NAWS and TTYPE; negotiated closes on both
	nawsAnswered, ttypeAnswered bool
	negotiated                  chan struct{}

	state readState
	verb  byte
	sb    []byte
	cr    bool
	data  []byte
	buf   [4096]byte

	// OnResize is called by the server side when the peer sends NAWS. Set it
	// before the first Read.
	OnResize func(width, height int)
}

var _ io.ReadWriteCloser = (*Conn)(nil)

// NewClient returns the side that connects to a telnet target.
func NewClient(c net.Conn, termType string) *Conn {
	return &Conn{
		rw:       c,
		role:     roleClient,
		termType: termType,
		willSent: make(map[byte]bool),
		doSent:   make(map[byte]bool),
		wontSent: make(map[byte]bool),
		dontSent: make(map[byte]bool),
	}
}

// NewServer returns the side that accepts a telnet client. It offers ECHO,
// SGA, NAWS and TTYPE immediately. rw needs no deadlines: the client side is
// bounded by the session, not by the connector.
func NewServer(rw io.ReadWriter) (*Conn, error) {
	conn := &Conn{
		rw:         rw,
		role:       roleServer,
		willSent:   make(map[byte]bool),
		doSent:     make(map[byte]bool),
		wontSent:   make(map[byte]bool),
		dontSent:   make(map[byte]bool),
		negotiated: make(chan struct{}),
	}
	conn.willSent[optEcho] = true
	conn.willSent[optSGA] = true
	conn.doSent[optNAWS] = true
	conn.doSent[optTTYPE] = true

	conn.mu.Lock()
	defer conn.mu.Unlock()

	for _, command := range [][]byte{
		commandBytes(WILL, optEcho),
		commandBytes(WILL, optSGA),
		commandBytes(DO, optNAWS),
		commandBytes(DO, optTTYPE),
	} {
		if _, err := rw.Write(command); err != nil {
			return nil, err
		}
	}

	return conn, nil
}

// Read returns application data with telnet commands removed.
func (c *Conn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	for {
		if len(c.data) > 0 {
			n := copy(p, c.data)
			c.data = c.data[n:]
			return n, nil
		}

		if err := c.takeWriteError(); err != nil {
			return 0, err
		}

		n, err := c.rw.Read(c.buf[:])
		if n > 0 {
			c.feed(c.buf[:n])
			continue
		}
		if err != nil {
			return 0, err
		}
	}
}

// Write sends application data and escapes IAC bytes.
func (c *Conn) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	escaped := make([]byte, 0, len(p))
	for _, b := range p {
		escaped = append(escaped, b)
		if b == IAC {
			escaped = append(escaped, IAC)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	n, err := c.rw.Write(escaped)
	if err != nil {
		return 0, err
	}
	if n != len(escaped) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}

// SetDeadline delegates to the underlying connection, if it has deadlines.
func (c *Conn) SetDeadline(t time.Time) error {
	if d, ok := c.rw.(interface{ SetDeadline(time.Time) error }); ok {
		return d.SetDeadline(t)
	}
	return nil
}

// Close closes the underlying connection, if it can be closed.
func (c *Conn) Close() error {
	if cl, ok := c.rw.(io.Closer); ok {
		return cl.Close()
	}
	return nil
}

func (c *Conn) feed(data []byte) {
	for _, b := range data {
		switch c.state {
		case stateData:
			if c.cr {
				c.cr = false
				if b == 0 {
					continue
				}
			}

			switch b {
			case IAC:
				c.state = stateIAC
			case '\r':
				c.data = append(c.data, b)
				c.cr = true
			default:
				c.data = append(c.data, b)
			}

		case stateIAC:
			switch b {
			case IAC:
				c.data = append(c.data, IAC)
				c.state = stateData
			case DO, DONT, WILL, WONT:
				c.verb = b
				c.state = stateVerb
			case SB:
				c.sb = c.sb[:0]
				c.state = stateSB
			default:
				c.state = stateData
			}

		case stateVerb:
			c.handleVerb(c.verb, b)
			c.state = stateData

		case stateSB:
			if b == IAC {
				c.state = stateSBIAC
				continue
			}
			if len(c.sb) < maxSubnegotiation {
				c.sb = append(c.sb, b)
			}

		case stateSBIAC:
			switch b {
			case SE:
				c.handleSubnegotiation(c.sb)
				c.sb = c.sb[:0]
				c.state = stateData
			case IAC:
				if len(c.sb) < maxSubnegotiation {
					c.sb = append(c.sb, IAC)
				}
				c.state = stateSB
			default:
				c.state = stateSB
			}
		}
	}
}
