package auditor

import (
	"io"

	"golang.org/x/crypto/ssh"
)

// logTunnel forwards bytes to an SSH channel and records them in a separate
// writer. The recorded stream is never written back to the SSH channel.
type logTunnel struct {
	channel ssh.Channel
	writer  io.Writer
}

// NewLogtunnel wraps an SSH channel. Bytes read from the wrapper come from the
// channel; bytes written to the wrapper are recorded in writer and forwarded to
// the channel. The writer must be independent from the channel, otherwise the
// recorded data is echoed back to the client.
func NewLogtunnel(channel ssh.Channel, writer io.Writer) io.ReadWriteCloser {
	return &logTunnel{
		channel: channel,
		writer:  writer,
	}
}

// Read reads data from the proxied SSH channel.
func (l *logTunnel) Read(data []byte) (int, error) {
	return l.channel.Read(data)
}

// Write records data and forwards it to the proxied SSH channel.
func (l *logTunnel) Write(data []byte) (int, error) {
	if l.writer != nil {
		if _, err := l.writer.Write(data); err != nil {
			log.Error(err).Msg("Failed to write log data")
		}
	}

	return l.channel.Write(data)
}

// Close closes the proxied SSH channel.
func (l *logTunnel) Close() error {
	return l.channel.Close()
}
