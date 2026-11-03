package auditor_test

import (
	"bytes"
	"encoding/base64"
	"io"
	"sync"
	"testing"

	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/core/scheme/services/ssh/auditor"
)

// fakeChannel is an in-memory ssh.Channel.
type fakeChannel struct {
	mu     sync.Mutex
	read   *bytes.Buffer
	write  *bytes.Buffer
	closed bool
}

func newFakeChannel(input string) *fakeChannel {
	return &fakeChannel{
		read:  bytes.NewBufferString(input),
		write: &bytes.Buffer{},
	}
}

func (c *fakeChannel) Read(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.read.Read(data)
}

func (c *fakeChannel) Write(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write.Write(data)
}

func (c *fakeChannel) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeChannel) CloseWrite() error { return nil }

func (c *fakeChannel) SendRequest(string, bool, []byte) (bool, error) { return false, nil }

func (c *fakeChannel) Stderr() io.ReadWriter { return &bytes.Buffer{} }

func (c *fakeChannel) written() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write.String()
}

var _ gossh.Channel = (*fakeChannel)(nil)

func TestLogtunnelReadDelegatesToChannel(t *testing.T) {
	channel := newFakeChannel("from-target")
	tunnel := auditor.NewLogtunnel(channel, io.Discard)

	buf := make([]byte, len("from-target"))
	n, err := tunnel.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got := string(buf[:n]); got != "from-target" {
		t.Fatalf("Read() = %q, want %q", got, "from-target")
	}
}

func TestLogtunnelWriteForwardsWithoutEcho(t *testing.T) {
	client := newFakeChannel("")
	target := newFakeChannel("")
	logs := &bytes.Buffer{}

	// Client to target direction. The wrapper writes into the target
	// channel and records into the log, never into the client channel.
	toTarget := auditor.NewLogtunnel(target, logs)
	if _, err := toTarget.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := target.written(); got != "hello" {
		t.Fatalf("target received %q, want %q", got, "hello")
	}
	if got := client.written(); got != "" {
		t.Fatalf("client received echoed data %q, want no data", got)
	}
	if got := logs.String(); got != "hello" {
		t.Fatalf("log writer received %q, want %q", got, "hello")
	}
}

func TestLogtunnelBothDirections(t *testing.T) {
	client := newFakeChannel("client-to-target")
	target := newFakeChannel("target-to-client")
	clientLogs := &bytes.Buffer{}
	targetLogs := &bytes.Buffer{}

	// The tunnel wrapping a channel is written to when traffic goes towards
	// that channel: client -> target writes into the target channel.
	toTarget := auditor.NewLogtunnel(target, targetLogs)
	toClient := auditor.NewLogtunnel(client, clientLogs)

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(toTarget, client)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(toClient, target)
		done <- struct{}{}
	}()

	<-done
	<-done

	if got := target.written(); got != "client-to-target" {
		t.Fatalf("target received %q, want %q", got, "client-to-target")
	}
	if got := client.written(); got != "target-to-client" {
		t.Fatalf("client received %q, want %q", got, "target-to-client")
	}
	if got := targetLogs.String(); got != "client-to-target" {
		t.Fatalf("target log = %q, want %q", got, "client-to-target")
	}
	if got := clientLogs.String(); got != "target-to-client" {
		t.Fatalf("client log = %q, want %q", got, "target-to-client")
	}
}

func TestLogtunnelRecordsRawPayload(t *testing.T) {
	target := newFakeChannel("")
	logs := &bytes.Buffer{}
	tunnel := auditor.NewLogtunnel(target, logs)

	payload := []byte{0x00, 0x01, 0xfe, 0xff}
	if _, err := tunnel.Write(payload); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if !bytes.Equal(logs.Bytes(), payload) {
		t.Fatalf("recorded payload = %v, want %v", logs.Bytes(), payload)
	}
}

func TestLogChannelWriteBinaryRecordsWithoutEcho(t *testing.T) {
	channel := newFakeChannel("")
	logChannel := &auditor.LogChannel{Channel: channel}

	payload := []byte{0x00, 0x01, 0xfe, 0xff}
	n, err := logChannel.WriteBinary(payload)
	if err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}
	if n != len(payload) {
		t.Fatalf("WriteBinary() = %d, want %d", n, len(payload))
	}
	if got := channel.written(); got != "" {
		t.Fatalf("client channel received %q, want no data", got)
	}
	if got := logChannel.FramesCount; got != 1 {
		t.Fatalf("frames count = %d, want 1", got)
	}

	frames := logChannel.Frames
	if len(frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(frames))
	}
	if frames[0].GetType() != "b" {
		t.Fatalf("frame type = %q, want b", frames[0].GetType())
	}
	if frames[0].GetDuration() == "" {
		t.Fatal("frame duration is empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(frames[0].GetScreen())
	if err != nil {
		t.Fatalf("frame screen is not base64: %v", err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("decoded screen = %v, want %v", decoded, payload)
	}
}

func TestLogChannelWriteStillWritesTerminalText(t *testing.T) {
	channel := newFakeChannel("")
	logChannel := &auditor.LogChannel{Channel: channel}

	if _, err := logChannel.Write([]byte("ls\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if got := channel.written(); got != "ls\n" {
		t.Fatalf("client channel received %q, want %q", got, "ls\n")
	}
	if len(logChannel.Frames) != 1 || logChannel.Frames[0].GetType() != "o" {
		t.Fatalf("frames = %+v, want one text frame", logChannel.Frames)
	}
}
