package auditor

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	auditmessage "github.com/werbot/werbot/internal/core/audit/proto/message"
	auditrpc "github.com/werbot/werbot/internal/core/audit/proto/rpc"
	"github.com/werbot/werbot/pkg/logger"
)

var log = logger.New()

// LogChannel is ...
type LogChannel struct {
	AuditID     string
	AccountID   string
	ProfileID   string
	SessionID   string
	ClientIP    string
	Channel     ssh.Channel
	FramesCount int32
	Frames      []*auditmessage.Record
	fullTime    time.Time

	mu          sync.Mutex
	grpcSession *grpc.ClientConn
	recordCount int32
}

// NewLogchannel is ...
func NewLogchannel(account *auditmessage.AddAudit_Request, channel ssh.Channel, grpcSession *grpc.ClientConn, recordCount int32) *LogChannel {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := auditrpc.NewAuditHandlersClient(grpcSession)
	auditData, err := rClient.AddAudit(ctx, &auditmessage.AddAudit_Request{
		AccountId: account.AccountId,
		ProfileId: account.ProfileId,
		Version:   2,
		ClientIp:  account.ClientIp,
		Session:   account.Session,
	})
	if err != nil {
		log.Error(err).Msg("Log channel create new channel failed")
		return nil
	}

	return &LogChannel{
		AuditID:     auditData.AuditId,
		AccountID:   account.AccountId,
		ProfileID:   account.ProfileId,
		SessionID:   account.Session,
		ClientIP:    account.ClientIp,
		Channel:     channel,
		fullTime:    time.Now(),
		FramesCount: 0,
		grpcSession: grpcSession,
		recordCount: recordCount,
	}
}

// Read is ...
// func (l *LogChannel) Read(data []byte) (int32, error) {
//	return l.Read(data)
// }

// Write records terminal text and forwards it to the client channel.
func (l *LogChannel) Write(data []byte) (int, error) {
	l.appendRecord(&auditmessage.Record{
		Duration: fmt.Sprintf("%.6f", l.Duration().Seconds()),
		Type:     "o",
		Screen:   string(data),
	})

	return l.Channel.Write(data)
}

// WriteBinary records raw tunnel bytes without forwarding them anywhere. The
// payload is stored base64 encoded because a screenshot record is text.
func (l *LogChannel) WriteBinary(data []byte) (int, error) {
	l.appendRecord(&auditmessage.Record{
		Duration: fmt.Sprintf("%.6f", l.Duration().Seconds()),
		Type:     "b",
		Screen:   base64.StdEncoding.EncodeToString(data),
	})

	return len(data), nil
}

// RecordWriter returns an io.Writer that records tunnel bytes without writing
// them to the channel.
func (l *LogChannel) RecordWriter() io.Writer {
	return recordWriter{log: l}
}

type recordWriter struct {
	log *LogChannel
}

func (w recordWriter) Write(data []byte) (int, error) {
	return w.log.WriteBinary(data)
}

func (l *LogChannel) appendRecord(record *auditmessage.Record) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.FramesCount++
	l.Frames = append(l.Frames, record)

	if l.FramesCount == l.recordCount {
		l.flushLocked()
	}
}

func (l *LogChannel) flushLocked() {
	if l.FramesCount == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := auditrpc.NewAuditHandlersClient(l.grpcSession)

	// TODO: переименовать названия на новые frame, Time, Type, Data
	if _, err := rClient.AddRecord(ctx, &auditmessage.AddRecord_Request{
		AuditId:   l.AuditID,
		ProfileId: l.ProfileID,
		Session:   l.SessionID,
		Records:   l.Frames,
	}); err != nil {
		log.Error(err).Msg("goroutine CreateRecord")
	}

	l.Frames = nil
	l.FramesCount = 0
}

// Close flushes pending frames and marks the audit as finished.
func (l *LogChannel) Close() error {
	l.mu.Lock()
	l.flushLocked()
	l.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := auditrpc.NewAuditHandlersClient(l.grpcSession)

	_, err := rClient.UpdateAudit(ctx, &auditmessage.UpdateAudit_Request{
		AuditId:   l.AuditID,
		ProfileId: l.ProfileID,
		Session:   l.SessionID,
		Duration:  fmt.Sprintf("%.6f", l.Duration().Seconds()),
	})
	if err != nil {
		log.Error(err).Msg("Logchannel close error")
	}

	return l.Channel.Close()
}

// Duration is ...
func (l *LogChannel) Duration() time.Duration {
	return time.Since(l.fullTime)
}
