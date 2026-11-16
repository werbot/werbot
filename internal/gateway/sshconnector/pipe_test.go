package sshconnector

import (
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPipeNoEcho(t *testing.T) {
	clientUser, client := net.Pipe()
	targetUser, target := net.Pipe()
	rec := &recSink{}
	piped := make(chan error, 1)
	go func() { piped <- pipe(client, target, rec) }()
	echoed := &lockedBuffer{}
	clientDone := make(chan struct{})
	go func() { _, _ = io.Copy(echoed, clientUser); close(clientDone) }()

	_, err := io.WriteString(clientUser, "client-to-target")
	require.NoError(t, err)
	got := make([]byte, len("client-to-target"))
	_, err = io.ReadFull(targetUser, got)
	require.NoError(t, err)
	require.Equal(t, "client-to-target", string(got))
	require.NoError(t, targetUser.Close()) // the target ends: the pipe closes the client side

	require.NoError(t, <-piped)
	<-clientDone
	require.Empty(t, echoed.String(), "the client got data back")
	in, out, _ := rec.get()
	require.Equal(t, "client-to-target", in)
	require.Empty(t, out)
}

// A panic in a relay ends the pipe with an error and closes both sides.
func TestPipeRelayPanic(t *testing.T) {
	targetUser, target := net.Pipe()
	err := pipe(panicConn{}, target, &recSink{})
	require.ErrorContains(t, err, "panic")
	_, err = targetUser.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF, "the target side is not closed")
}

type panicConn struct{ panicRW }

func (panicConn) Close() error { return nil }
