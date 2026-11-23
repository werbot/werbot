package gateway

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// Stream limits: the data of one frame (the server reads messages of up to
// 64 KiB) and how long a call outlives its context.
const (
	streamFrameBytes = 32 << 10
	streamCloseWait  = 5 * time.Second
)

// WebTicket is a stream ticket the server resolved: Mode "web" opens a
// terminal on Resource with Window, "observe" watches SessionID; ClientIP
// is the browser's.
type WebTicket struct {
	Mode, Resource string
	SessionID      uuid.UUID
	Window         Window
	ClientIP       netip.Addr
}

// relayStream is a browser stream through the server: Read returns data, or
// a window size when cols > 0; a close by the other side is a
// streamCloseError. Close is idempotent.
type relayStream interface {
	Read() (data []byte, cols, rows int, err error)
	Write(p []byte) error
	Resize(cols, rows int) error
	Close(reason string) error
}

// command runs hook with the id of a stream command; an id that does not
// parse is logged by its length only and the stream lives.
func (c *tunnelClient) command(name, id string, hook func(uuid.UUID)) {
	u, err := uuid.Parse(id)
	if err != nil {
		c.log.Warn("tunnel: ignoring "+name+" command", "id_len", len(id))
		return
	}
	if hook != nil {
		hook(u)
	}
}

// AuthenticateWebTicket resolves the user and the stream ticket of an
// open_stream; the server reads the ticket, it does not spend it. An
// unreachable server is ErrNotReady; an agent's answer is refused.
func (c *tunnelClient) AuthenticateWebTicket(ctx context.Context, ticket uuid.UUID) (User, WebTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	// client_ip is required; the browser's comes with the ticket
	resp, err := c.conn.Load().rpc.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
		By: &gatewayv1.AuthenticateUserRequest_WebTicket{WebTicket: ticket.String()}, ClientIp: netip.IPv4Unspecified().String()}))
	if err != nil {
		return User{}, WebTicket{}, controlErr(err)
	}
	u, err := userOf(resp.Msg)
	if err == nil {
		u, err = withAgent(u, nil, resp.Msg.GetAgent())
	}
	if err != nil {
		return User{}, WebTicket{}, err
	}
	wt, err := webTicketOf(resp.Msg.GetWeb())
	return u, wt, err
}

// webTicketOf is the ticket of an AuthenticateUser answer.
func webTicketOf(w *gatewayv1.WebTicket) (WebTicket, error) {
	if w == nil {
		return WebTicket{}, errors.New("authenticate web ticket: no ticket in response")
	}
	wt := WebTicket{Mode: w.GetMode(), Resource: w.GetResource(), Window: Window{Cols: int(w.GetCols()), Rows: int(w.GetRows())}}
	ip, err := netip.ParseAddr(w.GetClientIp())
	if err != nil {
		return WebTicket{}, errors.New("authenticate web ticket: invalid client_ip in response")
	}
	wt.ClientIP = ip.Unmap()
	if s := w.GetSessionId(); s != "" {
		if wt.SessionID, err = uuid.Parse(s); err != nil {
			return WebTicket{}, errors.New("authenticate web ticket: invalid session_id in response")
		}
	}
	return wt, nil
}

// OpenStream opens TunnelService.Stream for ticket: the server spends the
// ticket and joins the stream to the browser waiting for it. The stream
// ends with Close, a close by the other side or ctx.
func (c *tunnelClient) OpenStream(ctx context.Context, ticket uuid.UUID) (relayStream, error) {
	// The end of ctx ends the request side; the call itself lives up to
	// streamCloseWait longer, so the server still reads the last frames (the
	// close frame among them) and ends the response instead of losing them
	// to an aborted call.
	call, abort := context.WithCancel(context.WithoutCancel(ctx))
	st := c.conn.Load().rpc.Stream(call)
	context.AfterFunc(ctx, func() {
		_ = st.CloseRequest() // as in stream: unblocks the transport
		time.AfterFunc(streamCloseWait, abort)
	})
	s := &tunnelStream{st: st}
	if err := s.send(&gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Open{Open: &gatewayv1.StreamOpen{TicketId: ticket.String()}}}, false); err != nil {
		abort()
		_ = st.CloseRequest()
		_ = st.CloseResponse()
		return nil, err
	}
	return s, nil
}

// streamCloseError is the Read error of a stream the other side closed.
type streamCloseError struct{ reason string }

func (e streamCloseError) Error() string { return "stream closed: " + e.reason }

var errStreamClosed = errors.New("stream closed")

// tunnelStream is the relayStream of a Stream call; sends are serialized,
// the close frame is sent once and nothing after it.
type tunnelStream struct {
	st     *connect.BidiStreamForClient[gatewayv1.StreamRequest, gatewayv1.StreamResponse]
	mu     sync.Mutex
	closed bool
}

// Read returns the next data, or the next window with cols and rows > 0;
// other frames are skipped. A close or a failure ends the response side.
func (s *tunnelStream) Read() ([]byte, int, int, error) {
	for {
		m, err := s.st.Receive()
		if err != nil {
			_ = s.st.CloseResponse()
			return nil, 0, 0, err
		}
		switch f := m.GetFrame().GetFrame().(type) {
		case *gatewayv1.StreamFrame_Data:
			return f.Data, 0, 0, nil
		case *gatewayv1.StreamFrame_Resize:
			if cols, rows := int(f.Resize.GetCols()), int(f.Resize.GetRows()); cols > 0 && rows > 0 {
				return nil, cols, rows, nil
			}
		case *gatewayv1.StreamFrame_Close:
			_ = s.st.CloseResponse()
			return nil, 0, 0, streamCloseError{reason: f.Close.GetReason()}
		}
	}
}

// Write sends b in data frames of at most streamFrameBytes.
func (s *tunnelStream) Write(b []byte) error {
	for len(b) > 0 {
		n := min(len(b), streamFrameBytes)
		if err := s.send(&gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Data{Data: b[:n]}}, false); err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func (s *tunnelStream) Resize(cols, rows int) error {
	return s.send(&gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Resize{
		Resize: &gatewayv1.Resize{Cols: int32(cols), Rows: int32(rows)}}}, false) //nolint:gosec // terminal windows are small
}

// Close sends the close frame with reason and ends the request side.
func (s *tunnelStream) Close(reason string) error {
	err := s.send(&gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Close{Close: &gatewayv1.StreamClose{Reason: reason}}}, true)
	if errors.Is(err, errStreamClosed) {
		return nil
	}
	_ = s.st.CloseRequest()
	return err
}

// send sends f unless the stream is closed; last marks it closed.
func (s *tunnelStream) send(f *gatewayv1.StreamFrame, last bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errStreamClosed
	}
	s.closed = last
	return s.st.Send(&gatewayv1.StreamRequest{Frame: f})
}
