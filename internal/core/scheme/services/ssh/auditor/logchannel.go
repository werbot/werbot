package auditor

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	auditpb "github.com/werbot/werbot/internal/core/audit/proto/audit"
	"github.com/werbot/werbot/pkg/logger"
)

var log = logger.New()

// LogChannel is ...
type LogChannel struct {
	AuditID     string
	AccountID   string
	ClientIP    string
	Channel     ssh.Channel
	FramesCount int32
	Frames      []*auditpb.Record
	fullTime    time.Time

	grpcSession *grpc.ClientConn
	recordCount int32
}

// NewLogchannel is ...
func NewLogchannel(account *auditpb.AddAudit_Request, channel ssh.Channel, grpcSession *grpc.ClientConn, recordCount int32) *LogChannel {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := auditpb.NewAuditHandlersClient(grpcSession)
	auditData, err := rClient.AddAudit(ctx, &auditpb.AddAudit_Request{
		AccountId: account.AccountId,
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

// Write is ...
func (l *LogChannel) Write(data []byte) (int, error) {
	record := &auditpb.Record{}                                   // frame
	record.Duration = fmt.Sprintf("%.6f", l.Duration().Seconds()) // Time
	record.Type = "o"                                             // Type
	record.Screen = string(data)                                  // Data

	l.FramesCount++
	l.Frames = append(l.Frames, record)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := auditpb.NewAuditHandlersClient(l.grpcSession)

	if l.FramesCount == l.recordCount {
		// TODO: переименовать названия на новые frame, Time, Type, Data
		// go func() {
		_, err := rClient.AddRecord(ctx, &auditpb.AddRecord_Request{
			AuditId: l.AuditID,
			Records: l.Frames,
		})
		if err != nil {
			log.Error(err).Msg("goroutine CreateRecord")
		}
		// }()

		l.Frames = nil
		l.FramesCount = 0
	}

	return l.Channel.Write(data)
}

// Close is ...
func (l *LogChannel) Close() error {
	// TODO: вынести в отдельную функцию отправку дополнительного последнего сообщения рекордера о закрытии сессии
	//	if string(l.Frames[len(l.Frames)-1].Data) == "exit\r\n" {
	//		l.Frames = l.Frames[:len(l.Frames)-1]
	//	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := auditpb.NewAuditHandlersClient(l.grpcSession)

	if l.FramesCount > 0 {
		_, err := rClient.AddRecord(ctx, &auditpb.AddRecord_Request{
			AuditId: l.AuditID,
			Records: l.Frames,
		})
		if err != nil {
			log.Error(err).Msg("CreateRecord")
		}
		l.Frames = nil
		l.FramesCount = 0
	}

	_, err := rClient.UpdateAudit(ctx, &auditpb.UpdateAudit_Request{
		AuditId:  l.AuditID,
		Duration: fmt.Sprintf("%.6f", l.Duration().Seconds()),
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
