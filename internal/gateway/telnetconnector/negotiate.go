package telnetconnector

import "encoding/binary"

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

// Negotiated is closed once the client answered NAWS and TTYPE (with a value
// or WONT); the answers arrive only while someone reads. Server side only.
func (c *Conn) Negotiated() <-chan struct{} { return c.negotiated }

// answeredLocked notes a server-side answer and closes negotiated on the last one.
func (c *Conn) answeredLocked(option byte) {
	wasDone := c.nawsAnswered && c.ttypeAnswered
	switch option {
	case optNAWS:
		c.nawsAnswered = true
	case optTTYPE:
		c.ttypeAnswered = true
	}
	if !wasDone && c.nawsAnswered && c.ttypeAnswered {
		close(c.negotiated)
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
			_ = c.writeCommandLocked(commandBytes(WILL, option))
			if c.width > 0 || c.height > 0 {
				_ = c.writeCommandLocked(c.windowSizeBytesLocked())
			}
		case optTTYPE, optSGA:
			if c.willSent[option] {
				return
			}
			c.willSent[option] = true
			_ = c.writeCommandLocked(commandBytes(WILL, option))
		default:
			if c.wontSent[option] {
				return
			}
			c.wontSent[option] = true
			_ = c.writeCommandLocked(commandBytes(WONT, option))
		}

	case WILL:
		switch option {
		case optEcho, optSGA:
			if c.doSent[option] {
				return
			}
			c.doSent[option] = true
			_ = c.writeCommandLocked(commandBytes(DO, option))
		default:
			if c.dontSent[option] {
				return
			}
			c.dontSent[option] = true
			_ = c.writeCommandLocked(commandBytes(DONT, option))
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
			_ = c.writeCommandLocked(subnegotiationBytes(optTTYPE, []byte{ttypeSEND}))
		default:
			if c.dontSent[option] {
				return
			}
			c.dontSent[option] = true
			_ = c.writeCommandLocked(commandBytes(DONT, option))
		}

	case WONT:
		if option == optNAWS || option == optTTYPE {
			c.answeredLocked(option)
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
			_ = c.writeCommandLocked(commandBytes(WONT, option))
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
			_ = c.writeCommandLocked(subnegotiationBytes(optTTYPE, append([]byte{ttypeIS}, []byte(c.termType)...)))
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
			c.answeredLocked(optNAWS)
			onResize := c.OnResize
			c.mu.Unlock()
			if onResize != nil {
				onResize(width, height)
			}
			return

		case optTTYPE:
			if len(payload) > 0 && payload[0] == ttypeIS {
				c.termType = string(payload[1:])
				c.answeredLocked(optTTYPE)
			}
		}
		c.mu.Unlock()
	}
}

func (c *Conn) windowSizeBytesLocked() []byte {
	var payload [4]byte
	binary.BigEndian.PutUint16(payload[0:2], uint16(c.width))  //nolint:gosec // window sizes come from 16-bit NAWS or pty-req
	binary.BigEndian.PutUint16(payload[2:4], uint16(c.height)) //nolint:gosec // window sizes come from 16-bit NAWS or pty-req
	return subnegotiationBytes(optNAWS, payload[:])
}

func (c *Conn) sendWindowSizeLocked() error {
	return c.writeCommandLocked(c.windowSizeBytesLocked())
}

func (c *Conn) writeCommandLocked(command []byte) error {
	if c.writeErr != nil {
		return c.writeErr
	}
	if _, err := c.rw.Write(command); err != nil {
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
