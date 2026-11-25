package tunnel

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// Stream limits: one received message, one sent data frame and the window
// bounds a resize must keep.
const (
	streamReadMaxBytes = 64 << 10
	streamFrameBytes   = 32 << 10
	maxCols, maxRows   = 500, 200
)

// Pipe is one browser stream: Read returns data, or a window size when cols
// > 0; a close by the other side is an error with a CloseReason() string
// method. Close is idempotent.
type Pipe interface {
	Read() (data []byte, cols, rows int, err error)
	Write(p []byte) error
	Resize(cols, rows int) error
	Close(reason string) error
}

// CloseError is the Read error of a stream the other side closed with Reason.
type CloseError struct{ Reason string }

func (e CloseError) Error() string { return "stream closed: " + e.Reason }

// CloseReason is the reason the other side gave.
func (e CloseError) CloseReason() string { return e.Reason }

// Bridger joins a gateway's stream for a ticket to the browser waiting for
// it; it returns once the stream is done.
type Bridger interface {
	Bridge(ctx context.Context, gw Gateway, ticketID uuid.UUID, p Pipe) error
}

type tunnelStream = connect.BidiStream[gatewayv1.StreamRequest, gatewayv1.StreamResponse]

// mountStream registers Stream on its exact path, which wins over the
// service prefix: its messages are read up to streamReadMaxBytes.
func (h *Handler) mountStream(mux *http.ServeMux) {
	stream := connect.NewBidiStreamHandler(gatewayv1connect.TunnelServiceStreamProcedure, h.Stream,
		connect.WithReadMaxBytes(streamReadMaxBytes), connect.WithInterceptors(validate.NewInterceptor()),
		connect.WithSchema(gatewayv1.File_gateway_v1_tunnel_proto.Services().ByName("TunnelService").Methods().ByName("Stream")))
	mux.Handle(gatewayv1connect.TunnelServiceStreamProcedure, h.mtls(stream))
}

// Stream bridges a browser stream: the first frame opens it with a ticket,
// the relay spends the ticket and checks it belongs to the caller.
func (h *Handler) Stream(ctx context.Context, st *tunnelStream) error {
	g, err := caller(ctx)
	if err != nil {
		return err
	}
	if h.d.Relay == nil {
		return connect.NewError(connect.CodeUnimplemented, errors.New("browser streams are not served"))
	}
	first, err := st.Receive()
	if err != nil {
		return err
	}
	id, err := parseID(first.GetFrame().GetOpen().GetTicketId(), "open")
	if err != nil {
		return err
	}
	return apperr.ToConnect(h.d.Relay.Bridge(ctx, g, id, &streamPipe{recv: st.Receive, send: st.Send}))
}

// streamPipe is the Pipe of a Stream; sends are serialized, a close frame
// is sent once and nothing after it.
type streamPipe struct {
	recv   func() (*gatewayv1.StreamRequest, error)
	send   func(*gatewayv1.StreamResponse) error
	mu     sync.Mutex
	closed bool
}

var errPipeClosed = errors.New("stream pipe closed")

// Read returns the next data, an in-bounds resize or the close; other
// frames are skipped.
func (p *streamPipe) Read() ([]byte, int, int, error) {
	for {
		m, err := p.recv()
		if err != nil {
			return nil, 0, 0, err
		}
		switch f := m.GetFrame().GetFrame().(type) {
		case *gatewayv1.StreamFrame_Data:
			return f.Data, 0, 0, nil
		case *gatewayv1.StreamFrame_Resize:
			if c, r := int(f.Resize.GetCols()), int(f.Resize.GetRows()); c >= 1 && c <= maxCols && r >= 1 && r <= maxRows {
				return nil, c, r, nil
			}
		case *gatewayv1.StreamFrame_Close:
			return nil, 0, 0, CloseError{Reason: f.Close.GetReason()}
		}
	}
}

// Write sends b in data frames of at most streamFrameBytes.
func (p *streamPipe) Write(b []byte) error {
	for len(b) > 0 {
		n := min(len(b), streamFrameBytes)
		if err := p.sendFrame(&gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Data{Data: b[:n]}}, false); err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func (p *streamPipe) Resize(cols, rows int) error {
	return p.sendFrame(&gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Resize{
		Resize: &gatewayv1.Resize{Cols: int32(cols), Rows: int32(rows)}}}, false) //nolint:gosec // the relay bounds windows
}

func (p *streamPipe) Close(reason string) error {
	err := p.sendFrame(&gatewayv1.StreamFrame{Frame: &gatewayv1.StreamFrame_Close{Close: &gatewayv1.StreamClose{Reason: reason}}}, true)
	if errors.Is(err, errPipeClosed) {
		return nil
	}
	return err
}

// sendFrame sends f unless the pipe is closed; last marks it closed.
func (p *streamPipe) sendFrame(f *gatewayv1.StreamFrame, last bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errPipeClosed
	}
	p.closed = last
	return p.send(&gatewayv1.StreamResponse{Frame: f})
}
