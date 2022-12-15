package logtunnel

import (
	"errors"
	"io"

	"github.com/werbot/werbot/internal/logger"
	"golang.org/x/crypto/ssh"
)

var log = logger.New("ssh/logtunnel")

type logTunnel struct {
	host    string
	channel ssh.Channel
	writer  io.WriteCloser
}

// ForwardData ...
type ForwardData struct {
	DestinationHost string
	DestinationPort uint32
	SourceHost      string
	SourcePort      uint32
}

// NewLogtunnel is ...
func NewLogtunnel(channel ssh.Channel, writer io.WriteCloser, host string) io.ReadWriteCloser {
	return &logTunnel{
		host:    host,
		channel: channel,
		writer:  writer,
	}
}

// Read is ...
func (l *logTunnel) Read(data []byte) (int, error) {
	return 0, errors.New("logTunnel.Read is not implemented")
}

// Write is ...
func (l *logTunnel) Write(data []byte) (int, error) {
	writeHeader(l.writer, len(data)+len(l.host+": "))
	if _, err := l.writer.Write([]byte(l.host + ": ")); err != nil {
		log.Error(err).Msg("Failed to write log header")
	}
	if _, err := l.writer.Write(data); err != nil {
		log.Error(err).Msg("Failed to write log header")
	}
	return l.channel.Write(data)
}

// Close is ...
func (l *logTunnel) Close() error {
	l.writer.Close()
	return l.channel.Close()
}
