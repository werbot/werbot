package auditor

import (
	"encoding/binary"
	"io"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

// logTunnel forwards bytes to an SSH channel and records them in a separate
// writer. The recorded stream is never written back to the SSH channel.
type logTunnel struct {
	host    string
	channel ssh.Channel
	writer  io.Writer
}

// ForwardData ...
type ForwardData struct {
	DestinationHost string
	DestinationPort uint32
	SourceHost      string
	SourcePort      uint32
}

// NewLogtunnel wraps an SSH channel. Bytes read from the wrapper come from the
// channel; bytes written to the wrapper are recorded in writer and forwarded to
// the channel. The writer must be independent from the channel, otherwise the
// recorded data is echoed back to the client.
func NewLogtunnel(channel ssh.Channel, writer io.Writer, host string) io.ReadWriteCloser {
	return &logTunnel{
		host:    host,
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
		writeHeader(l.writer, len(data)+len(l.host+": "))
		if _, err := l.writer.Write([]byte(l.host + ": ")); err != nil {
			log.Error(err).Msg("Failed to write log header")
		}
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

func writeHeader(fd io.Writer, length int) {
	t := time.Now()
	tv := syscall.NsecToTimeval(t.UnixNano())

	if err := binary.Write(fd, binary.LittleEndian, int32(tv.Sec)); err != nil { //nolint:gosec // G115: Unix seconds fit in int32 until 2038
		log.Error(err).Msg("Failed to write log header")
	}
	if err := binary.Write(fd, binary.LittleEndian, tv.Usec); err != nil {
		log.Error(err).Msg("Failed to write log header")
	}
	if err := binary.Write(fd, binary.LittleEndian, int32(length)); err != nil { //nolint:gosec // G115: log record length is bounded by the read buffer
		log.Error(err).Msg("Failed to write log header")
	}
}
