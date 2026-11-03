package auditor_test

import (
	"bytes"
	"io"
	"strings"
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
	tunnel := auditor.NewLogtunnel(channel, io.Discard, "target")

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
	toTarget := auditor.NewLogtunnel(target, logs, "target")
	if _, err := toTarget.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := target.written(); got != "hello" {
		t.Fatalf("target received %q, want %q", got, "hello")
	}
	if got := client.written(); got != "" {
		t.Fatalf("client received echoed data %q, want no data", got)
	}
	if !strings.Contains(logs.String(), "target: hello") {
		t.Fatalf("log writer did not receive tunnel data: %q", logs.String())
	}
}

func TestLogtunnelBothDirections(t *testing.T) {
	client := newFakeChannel("client-to-target")
	target := newFakeChannel("target-to-client")
	clientLogs := &bytes.Buffer{}
	targetLogs := &bytes.Buffer{}

	// The tunnel wrapping a channel is written to when traffic goes towards
	// that channel: client -> target writes into the target channel.
	toTarget := auditor.NewLogtunnel(target, targetLogs, "target")
	toClient := auditor.NewLogtunnel(client, clientLogs, "client")

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
	if !strings.Contains(targetLogs.String(), "target: ") {
		t.Fatalf("target log is empty: %q", targetLogs.String())
	}
	if !strings.Contains(clientLogs.String(), "client: ") {
		t.Fatalf("client log is empty: %q", clientLogs.String())
	}
}
