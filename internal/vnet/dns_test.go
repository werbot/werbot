package vnet

import (
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

func testResolver() *resolver {
	r := newResolver("acme.werbot")
	r.set(map[string]netip.Addr{"pg.acme.werbot.": netip.MustParseAddr("100.64.1.7")})
	return r
}

func query(t *testing.T, id uint16, qs ...dnsmessage.Question) []byte {
	t.Helper()
	b, err := (&dnsmessage.Message{Header: dnsmessage.Header{ID: id, RecursionDesired: true}, Questions: qs}).Pack()
	require.NoError(t, err)
	return b
}

func q(name string, typ dnsmessage.Type) dnsmessage.Question {
	return dnsmessage.Question{Name: dnsmessage.MustNewName(name), Type: typ, Class: dnsmessage.ClassINET}
}

func ask(t *testing.T, r *resolver, qs ...dnsmessage.Question) dnsmessage.Message {
	t.Helper()
	out, ok := r.answer(query(t, 7, qs...))
	require.True(t, ok)
	var m dnsmessage.Message
	require.NoError(t, m.Unpack(out))
	require.Equal(t, uint16(7), m.ID)
	require.True(t, m.Response)
	return m
}

func TestDNSAnswers(t *testing.T) {
	r := testResolver()
	for _, name := range []string{"pg.acme.werbot.", "PG.Acme.WERBOT."} {
		m := ask(t, r, q(name, dnsmessage.TypeA))
		require.Equal(t, dnsmessage.RCodeSuccess, m.RCode)
		require.True(t, m.Authoritative)
		require.Len(t, m.Answers, 1)
		require.Equal(t, uint32(60), m.Answers[0].Header.TTL)
		require.Equal(t, [4]byte{100, 64, 1, 7}, m.Answers[0].Body.(*dnsmessage.AResource).A)
	}
	m := ask(t, r, q("pg.acme.werbot.", dnsmessage.TypeAAAA))
	require.Equal(t, dnsmessage.RCodeSuccess, m.RCode)
	require.Empty(t, m.Answers)
	require.Equal(t, dnsmessage.RCodeNameError, ask(t, r, q("nope.acme.werbot.", dnsmessage.TypeA)).RCode)
	m = ask(t, r, q("acme.werbot.", dnsmessage.TypeA))
	require.Equal(t, dnsmessage.RCodeSuccess, m.RCode)
	require.Empty(t, m.Answers)
	require.Equal(t, dnsmessage.RCodeRefused, ask(t, r, q("example.com.", dnsmessage.TypeA)).RCode)
	require.Equal(t, dnsmessage.RCodeRefused, ask(t, r, q("evilacme.werbot.", dnsmessage.TypeA)).RCode)
	require.Equal(t, dnsmessage.RCodeFormatError, ask(t, r, q("pg.acme.werbot.", dnsmessage.TypeA), q("pg.acme.werbot.", dnsmessage.TypeAAAA)).RCode)

	_, ok := r.answer([]byte{1, 2, 3})
	require.False(t, ok)
}

func TestDNSOverTCP(t *testing.T) {
	r := testResolver()
	srv, cli := net.Pipe()
	done := make(chan struct{})
	go func() { serveDNSTCP(srv, r); close(done) }()

	for _, name := range []string{"pg.acme.werbot.", "nope.acme.werbot."} {
		qb := query(t, 9, q(name, dnsmessage.TypeA))
		_, err := cli.Write([]byte{0, byte(len(qb))}) //nolint:gosec // test queries are under 256 bytes
		require.NoError(t, err)
		_, err = cli.Write(qb)
		require.NoError(t, err)
		var l [2]byte
		_, err = io.ReadFull(cli, l[:])
		require.NoError(t, err)
		got := make([]byte, binary.BigEndian.Uint16(l[:]))
		_, err = io.ReadFull(cli, got)
		require.NoError(t, err)
		want, _ := r.answer(qb)
		require.Equal(t, want, got)
	}
	require.NoError(t, cli.Close())
	<-done
}
