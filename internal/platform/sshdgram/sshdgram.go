// Package sshdgram is the wire format of Werbot's SSH extensions for port
// access, shared by the gateway and werbot connect: the datagram@werbot.com
// channel and the resource@werbot.com global request. It uses only the
// standard library.
package sshdgram

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// ChannelType carries the datagrams of one session, one frame each.
	ChannelType = "datagram@werbot.com"
	// ResourceRequest is the global request that answers ResourceInfo as JSON.
	ResourceRequest = "resource@werbot.com"
	// MaxPayload is the largest datagram a frame carries: its length is a uint16.
	MaxPayload = 65535
)

// headerLen is flow uint32 | len uint16.
const headerLen = 6

// OpenData is the channel open data of ChannelType, laid out like direct-tcpip
// (RFC 4254 §7.2), so ssh.Marshal and ssh.Unmarshal read and write it.
type OpenData struct {
	Host       string
	Port       uint32
	OriginAddr string
	OriginPort uint32
}

// WriteFrame writes p as one frame of flow: flow uint32 | len uint16 | p,
// big-endian. A frame is two writes, so writers of one channel take turns.
func WriteFrame(w io.Writer, flow uint32, p []byte) error {
	if len(p) > MaxPayload {
		return fmt.Errorf("sshdgram: datagram of %d bytes is over %d", len(p), MaxPayload)
	}
	var h [headerLen]byte
	binary.BigEndian.PutUint32(h[:], flow)
	binary.BigEndian.PutUint16(h[4:], uint16(len(p))) //nolint:gosec // checked above
	if _, err := w.Write(h[:]); err != nil {
		return err
	}
	if len(p) == 0 {
		return nil
	}
	_, err := w.Write(p)
	return err
}

// ReadFrame reads one frame into buf and returns its flow and length. A frame
// longer than buf is read to its end and dropped with io.ErrShortBuffer, so the
// next frame stays readable. A clean end between frames is io.EOF; a frame cut
// short is io.ErrUnexpectedEOF. It never reads past the frame's length (at most
// MaxPayload) and allocates nothing.
func ReadFrame(r io.Reader, buf []byte) (flow uint32, n int, err error) {
	var h [headerLen]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, 0, err
	}
	flow = binary.BigEndian.Uint32(h[:])
	n = int(binary.BigEndian.Uint16(h[4:]))
	if n > len(buf) {
		if _, err := io.CopyN(io.Discard, r, int64(n)); err != nil {
			return flow, 0, cut(err)
		}
		return flow, 0, io.ErrShortBuffer
	}
	if _, err := io.ReadFull(r, buf[:n]); err != nil {
		return flow, 0, cut(err)
	}
	return flow, n, nil
}

// cut turns an end inside a frame into io.ErrUnexpectedEOF.
func cut(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// ResourceInfo answers ResourceRequest: the resource type, its session kinds
// and its target ports.
type ResourceInfo struct {
	Type  string   `json:"type"`
	Kinds []string `json:"kinds"`
	Ports []Port   `json:"ports"`
}

// Port is a target port or, with EndPort above 0, the range Port–EndPort.
type Port struct {
	Port    uint16 `json:"port"`
	EndPort uint16 `json:"end_port"`
}
