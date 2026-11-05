package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net"
	"sync"
	"testing"

	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	auditmessage "github.com/werbot/werbot/internal/core/audit/proto/message"
	auditrpc "github.com/werbot/werbot/internal/core/audit/proto/rpc"
)

const (
	testSchemeMemberID = "44444444-4444-4444-4444-444444444444"
	testProfileID      = "55555555-5555-5555-5555-555555555555"
)

type auditRecorder struct {
	auditrpc.UnimplementedAuditHandlersServer

	mu      sync.Mutex
	audits  []*auditmessage.AddAudit_Request
	records []*auditmessage.Record
	updates []*auditmessage.UpdateAudit_Request
}

func (r *auditRecorder) AddAudit(_ context.Context, in *auditmessage.AddAudit_Request) (*auditmessage.AddAudit_Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.audits = append(r.audits, in)
	return &auditmessage.AddAudit_Response{AuditId: "11111111-1111-1111-1111-111111111111"}, nil
}

func (r *auditRecorder) AddRecord(_ context.Context, in *auditmessage.AddRecord_Request) (*auditmessage.AddRecord_Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, in.GetRecords()...)
	return &auditmessage.AddRecord_Response{}, nil
}

func (r *auditRecorder) UpdateAudit(_ context.Context, in *auditmessage.UpdateAudit_Request) (*auditmessage.UpdateAudit_Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, in)
	return &auditmessage.UpdateAudit_Response{}, nil
}

func startAuditRecorder(t *testing.T) *auditRecorder {
	t.Helper()

	recorder := &auditRecorder{}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	auditrpc.RegisterAuditHandlersServer(server, recorder)

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	previous := app.grpc
	app.grpc = conn
	t.Cleanup(func() { app.grpc = previous })

	return recorder
}

type tunnelChannel struct {
	mu    sync.Mutex
	read  *bytes.Buffer
	write bytes.Buffer
	block <-chan struct{}
}

func newTunnelChannel(input string, block <-chan struct{}) *tunnelChannel {
	return &tunnelChannel{
		read:  bytes.NewBufferString(input),
		block: block,
	}
}

func (c *tunnelChannel) Read(data []byte) (int, error) {
	if c.block != nil {
		<-c.block
		return 0, io.EOF
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.read.Read(data)
}

func (c *tunnelChannel) Write(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write.Write(data)
}

func (c *tunnelChannel) Close() error { return nil }

func (c *tunnelChannel) CloseWrite() error { return nil }

func (c *tunnelChannel) SendRequest(string, bool, []byte) (bool, error) { return false, nil }

func (c *tunnelChannel) Stderr() io.ReadWriter { return &bytes.Buffer{} }

func (c *tunnelChannel) written() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write.String()
}

var _ gossh.Channel = (*tunnelChannel)(nil)

type tunnelNewChannel struct{}

func (tunnelNewChannel) Accept() (gossh.Channel, <-chan *gossh.Request, error) {
	return nil, nil, nil
}

func (tunnelNewChannel) Reject(gossh.RejectionReason, string) error { return nil }

func (tunnelNewChannel) Read([]byte) (int, error) { return 0, io.EOF }

//nolint:unparam // stub required by gossh.NewChannel
func (tunnelNewChannel) Write(data []byte) (int, error) {
	return len(data), nil
}

func (tunnelNewChannel) Close() error { return nil }

func (tunnelNewChannel) ChannelType() string { return channelTypeDirectTCPIP }

func (tunnelNewChannel) ExtraData() []byte { return nil }

func TestPipeDirectTCPIPRecordsTunnelWithoutEcho(t *testing.T) {
	recorder := startAuditRecorder(t)

	release := make(chan struct{})
	client := newTunnelChannel("client-to-target", nil)
	target := newTunnelChannel("", release)
	lreqs := make(chan *gossh.Request)
	rreqs := make(chan *gossh.Request)

	err := pipe(lreqs, rreqs, client, target, tunnelNewChannel{}, sessionConfig{
		AccountID: testSchemeMemberID,
		ProfileID: testProfileID,
		ClientIP:  "8.8.8.8",
		SessionID: "session-tunnel",
	})
	if err != nil {
		t.Fatalf("pipe() error = %v", err)
	}
	close(release)

	if got := target.written(); got != "client-to-target" {
		t.Fatalf("target received %q, want %q", got, "client-to-target")
	}
	if got := client.written(); got != "" {
		t.Fatalf("client received echoed data %q, want no data", got)
	}

	recorder.mu.Lock()
	audits := append([]*auditmessage.AddAudit_Request(nil), recorder.audits...)
	records := append([]*auditmessage.Record(nil), recorder.records...)
	updates := append([]*auditmessage.UpdateAudit_Request(nil), recorder.updates...)
	recorder.mu.Unlock()

	if len(audits) != 1 {
		t.Fatalf("audits = %d, want 1", len(audits))
	}
	if got := audits[0].GetAccountId(); got != testSchemeMemberID {
		t.Fatalf("audit account_id = %q, want scheme_member id %q", got, testSchemeMemberID)
	}
	if got := audits[0].GetProfileId(); got != testProfileID {
		t.Fatalf("audit profile_id = %q, want %q", got, testProfileID)
	}
	if len(updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(updates))
	}
	if got := updates[0].GetProfileId(); got != testProfileID {
		t.Fatalf("update profile_id = %q, want %q", got, testProfileID)
	}

	var binaryRecords []*auditmessage.Record
	for _, record := range records {
		if record.GetType() == "b" {
			binaryRecords = append(binaryRecords, record)
		}
	}
	if len(binaryRecords) != 1 {
		t.Fatalf("binary records = %d, want 1", len(binaryRecords))
	}

	decoded, err := base64.StdEncoding.DecodeString(binaryRecords[0].GetScreen())
	if err != nil {
		t.Fatalf("binary record is not base64: %v", err)
	}
	if got := string(decoded); got != "client-to-target" {
		t.Fatalf("binary record = %q, want %q", got, "client-to-target")
	}
}
