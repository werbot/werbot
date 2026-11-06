// Package telnet implements the telnet transport pieces avocado needs:
// IAC escaping, option negotiation, window-size updates and automatic login.
package telnet

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"
)

// Telnet protocol commands and options used by avocado.
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

// Conn wraps a net.Conn and hides telnet negotiation from its readers.
// Read returns only application data; Write escapes IAC.
type Conn struct {
	netConn net.Conn
	role    role

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

	state readState
	verb  byte
	sb    []byte
	cr    bool
	data  []byte
	buf   [4096]byte

	// OnResize is called by the server side when the peer sends NAWS.
	OnResize func(width, height int)
}

var _ io.ReadWriteCloser = (*Conn)(nil)

// NewClient returns the side that connects to a telnet target.
func NewClient(c net.Conn, termType string) *Conn {
	return &Conn{
		netConn:  c,
		role:     roleClient,
		termType: termType,
		willSent: make(map[byte]bool),
		doSent:   make(map[byte]bool),
		wontSent: make(map[byte]bool),
		dontSent: make(map[byte]bool),
	}
}

// NewServer returns the side that accepts a telnet client. It offers ECHO,
// SGA, NAWS and TTYPE immediately.
func NewServer(c net.Conn) (*Conn, error) {
	conn := &Conn{
		netConn:  c,
		role:     roleServer,
		willSent: make(map[byte]bool),
		doSent:   make(map[byte]bool),
		wontSent: make(map[byte]bool),
		dontSent: make(map[byte]bool),
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
		if _, err := c.Write(command); err != nil {
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

		n, err := c.netConn.Read(c.buf[:])
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

	n, err := c.netConn.Write(escaped)
	if err != nil {
		return 0, err
	}
	if n != len(escaped) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}

// SetWindowSize stores the client window size and sends NAWS once the peer
// has agreed to the option.
func (c *Conn) SetWindowSize(width, height int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.width = width
	c.height = height

	if c.role != roleClient || !c.nawsAgreed {
		return nil
	}
	return c.sendWindowSizeLocked()
}

// Size returns the last window size negotiated by the server side.
func (c *Conn) Size() (width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.width, c.height
}

// TermType returns the terminal type reported by the client.
func (c *Conn) TermType() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.termType
}

// SetDeadline delegates to the underlying connection.
func (c *Conn) SetDeadline(t time.Time) error {
	return c.netConn.SetDeadline(t)
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.netConn.Close()
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

func (c *Conn) handleVerb(verb, option byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.role {
	case roleClient:
		c.handleClientVerbLocked(verb, option)
	case roleServer:
		c.handleServerVerbLocked(verb, option)
	}
}

func (c *Conn) handleClientVerbLocked(verb, option byte) {
	switch verb {
	case DO:
		switch option {
		case optNAWS:
			if c.willSent[option] {
				return
			}
			c.willSent[option] = true
			c.nawsAgreed = true
			c.writeCommandLocked(commandBytes(WILL, option))
			if c.width > 0 || c.height > 0 {
				c.writeCommandLocked(c.windowSizeBytesLocked())
			}
		case optTTYPE:
			if c.willSent[option] {
				return
			}
			c.willSent[option] = true
			c.writeCommandLocked(commandBytes(WILL, option))
		case optSGA:
			if c.willSent[option] {
				return
			}
			c.willSent[option] = true
			c.writeCommandLocked(commandBytes(WILL, option))
		default:
			if c.wontSent[option] {
				return
			}
			c.wontSent[option] = true
			c.writeCommandLocked(commandBytes(WONT, option))
		}

	case WILL:
		switch option {
		case optEcho, optSGA:
			if c.doSent[option] {
				return
			}
			c.doSent[option] = true
			c.writeCommandLocked(commandBytes(DO, option))
		default:
			if c.dontSent[option] {
				return
			}
			c.dontSent[option] = true
			c.writeCommandLocked(commandBytes(DONT, option))
		}
	}
}

func (c *Conn) handleServerVerbLocked(verb, option byte) {
	switch verb {
	case WILL:
		switch option {
		case optNAWS:
			return
		case optTTYPE:
			if c.sendTTYPE {
				return
			}
			c.sendTTYPE = true
			c.writeCommandLocked(subnegotiationBytes(optTTYPE, []byte{ttypeSEND}))
		default:
			if c.dontSent[option] {
				return
			}
			c.dontSent[option] = true
			c.writeCommandLocked(commandBytes(DONT, option))
		}

	case DO:
		switch option {
		case optEcho, optSGA:
			return
		default:
			if c.wontSent[option] {
				return
			}
			c.wontSent[option] = true
			c.writeCommandLocked(commandBytes(WONT, option))
		}
	}
}

func (c *Conn) handleSubnegotiation(data []byte) {
	if len(data) < 2 {
		return
	}
	option, payload := data[0], data[1:]

	c.mu.Lock()

	switch c.role {
	case roleClient:
		if option == optTTYPE && len(payload) > 0 && payload[0] == ttypeSEND {
			c.writeCommandLocked(subnegotiationBytes(optTTYPE, append([]byte{ttypeIS}, []byte(c.termType)...)))
		}
		c.mu.Unlock()

	case roleServer:
		switch option {
		case optNAWS:
			if len(payload) < 4 {
				c.mu.Unlock()
				return
			}
			width := int(binary.BigEndian.Uint16(payload[0:2]))
			height := int(binary.BigEndian.Uint16(payload[2:4]))
			c.width = width
			c.height = height
			onResize := c.OnResize
			c.mu.Unlock()
			if onResize != nil {
				onResize(width, height)
			}
			return

		case optTTYPE:
			if len(payload) > 0 && payload[0] == ttypeIS {
				c.termType = string(payload[1:])
			}
		}
		c.mu.Unlock()
	}
}

func (c *Conn) windowSizeBytesLocked() []byte {
	var payload [4]byte
	binary.BigEndian.PutUint16(payload[0:2], uint16(c.width))  //nolint:gosec // window size comes from pty-req
	binary.BigEndian.PutUint16(payload[2:4], uint16(c.height)) //nolint:gosec // window size comes from pty-req
	return subnegotiationBytes(optNAWS, payload[:])
}

func (c *Conn) sendWindowSizeLocked() error {
	return c.writeCommandLocked(c.windowSizeBytesLocked())
}

func (c *Conn) writeCommandLocked(command []byte) error {
	if c.writeErr != nil {
		return c.writeErr
	}
	if _, err := c.netConn.Write(command); err != nil {
		c.writeErr = err
		return err
	}
	return nil
}

func (c *Conn) takeWriteError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.writeErr
	c.writeErr = nil
	return err
}

func commandBytes(verb, option byte) []byte {
	return []byte{IAC, verb, option}
}

func subnegotiationBytes(option byte, payload []byte) []byte {
	out := make([]byte, 0, 5+len(payload))
	out = append(out, IAC, SB, option)
	for _, b := range payload {
		out = append(out, b)
		if b == IAC {
			out = append(out, IAC)
		}
	}
	out = append(out, IAC, SE)
	return out
}
