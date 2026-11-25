package vnet

import (
	"encoding/binary"
	"io"
	"math"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

const tcpIdle = 10 * time.Second

// resolver answers queries for names under suffix from an atomically replaced table.
type resolver struct {
	suffix string // lower case, ends with "."
	names  atomic.Pointer[map[string]netip.Addr]
}

func newResolver(suffix string) *resolver {
	r := &resolver{suffix: strings.ToLower(strings.TrimSuffix(suffix, ".")) + "."}
	r.set(nil)
	return r
}

// set replaces the table; keys are lower-case fully qualified names ending with ".".
func (r *resolver) set(names map[string]netip.Addr) {
	if names == nil {
		names = map[string]netip.Addr{}
	}
	r.names.Store(&names)
}

func (r *resolver) inSuffix(name string) bool {
	return name == r.suffix || strings.HasSuffix(name, "."+r.suffix)
}

// answer builds the reply for one query. false means the query is not answered
// (the header does not parse or the message is itself a response).
func (r *resolver) answer(query []byte) ([]byte, bool) {
	var p dnsmessage.Parser
	h, err := p.Start(query)
	if err != nil || h.Response {
		return nil, false
	}
	reply := dnsmessage.Message{Header: dnsmessage.Header{ID: h.ID, Response: true, OpCode: h.OpCode, RecursionDesired: h.RecursionDesired}}
	qs, err := p.AllQuestions()
	switch {
	case err != nil || len(qs) != 1:
		reply.RCode = dnsmessage.RCodeFormatError
	case h.OpCode != 0:
		reply.Questions, reply.RCode = qs, dnsmessage.RCodeNotImplemented
	default:
		reply.Questions = qs
		r.fill(&reply, qs[0])
	}
	out, err := reply.Pack()
	return out, err == nil
}

func (r *resolver) fill(reply *dnsmessage.Message, q dnsmessage.Question) {
	name := strings.ToLower(q.Name.String())
	if !r.inSuffix(name) {
		reply.RCode = dnsmessage.RCodeRefused
		return
	}
	reply.Authoritative = true
	ip, known := (*r.names.Load())[name]
	switch {
	case name == r.suffix:
	case !known:
		reply.RCode = dnsmessage.RCodeNameError
	case q.Type == dnsmessage.TypeA && ip.Is4():
		reply.Answers = []dnsmessage.Resource{{
			Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: TTL},
			Body:   &dnsmessage.AResource{A: ip.As4()},
		}}
	}
}

// serveDNSTCP serves RFC 7766 framed queries until the peer closes or stays idle.
func serveDNSTCP(c net.Conn, r *resolver) {
	defer c.Close()
	var lenBuf [2]byte
	for {
		_ = c.SetReadDeadline(time.Now().Add(tcpIdle))
		if _, err := io.ReadFull(c, lenBuf[:]); err != nil {
			return
		}
		query := make([]byte, binary.BigEndian.Uint16(lenBuf[:]))
		if _, err := io.ReadFull(c, query); err != nil {
			return
		}
		resp, ok := r.answer(query)
		if !ok || len(resp) > math.MaxUint16 {
			return
		}
		frame := binary.BigEndian.AppendUint16(nil, uint16(len(resp))) //nolint:gosec // length checked above
		if _, err := c.Write(append(frame, resp...)); err != nil {
			return
		}
	}
}
