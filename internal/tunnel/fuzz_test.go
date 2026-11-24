package tunnel

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// FuzzStreamFrame: a StreamRequest of any bytes never panics the first-frame
// parse of Stream or streamPipe.Read; an open ticket parses exactly when it is
// a UUID; Read returns data unchanged, a resize only inside the window, a
// close as CloseError with its reason, and skips every other frame; Close
// twice sends one close frame.
func FuzzStreamFrame(f *testing.F) {
	frames := []*gatewayv1.StreamFrame{
		openFrame(uuid.NewString()), openFrame("not-a-uuid"), openFrame(""),
		dataFrame(nil), dataFrame([]byte("x")), resizeFrame(0, 0), resizeFrame(80, 24),
		resizeFrame(-1, -1), resizeFrame(1<<30, 1<<30), resizeFrame(500, 200), resizeFrame(501, 200),
		closeFrame(strings.Repeat("r", 4096)), closeFrame(""), {},
	}
	for _, fr := range frames {
		f.Add(marshalRequest(f, fr))
	}
	f.Add(marshalRequest(f, dataFrame(bytes.Repeat([]byte{'d'}, 64<<10-16))))
	f.Add(append(marshalRequest(f, closeFrame("a")), 0x98, 0x7f, 0x01)) // unknown field 2043
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		var req gatewayv1.StreamRequest
		if proto.Unmarshal(raw, &req) != nil {
			t.Skip()
		}
		checkOpen(t, &req)
		checkRead(t, req.GetFrame())
		checkCloseOnce(t)
	})
}

func marshalRequest(f *testing.F, fr *gatewayv1.StreamFrame) []byte {
	f.Helper()
	b, err := proto.Marshal(&gatewayv1.StreamRequest{Frame: fr})
	if err != nil {
		f.Fatal(err)
	}
	return b
}

// checkOpen: parseID of the open ticket succeeds exactly for a UUID.
func checkOpen(t *testing.T, req *gatewayv1.StreamRequest) {
	t.Helper()
	ticket := req.GetFrame().GetOpen().GetTicketId()
	_, err := parseID(ticket, "open")
	_, perr := uuid.Parse(ticket)
	if (err == nil) != (perr == nil) {
		t.Fatalf("parseID(%q) = %v", ticket, err)
	}
	if err != nil && connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("parseID(%q) code = %v, want invalid_argument", ticket, connect.CodeOf(err))
	}
}

// feed returns the frame, then a close "end", then io.EOF.
func feed(fr *gatewayv1.StreamFrame) func() (*gatewayv1.StreamRequest, error) {
	queue := []*gatewayv1.StreamFrame{fr, closeFrame("end")}
	return func() (*gatewayv1.StreamRequest, error) {
		if len(queue) == 0 {
			return nil, io.EOF
		}
		fr := queue[0]
		queue = queue[1:]
		return &gatewayv1.StreamRequest{Frame: fr}, nil
	}
}

// checkRead: one Read of fr, whose next frame is a close "end".
func checkRead(t *testing.T, fr *gatewayv1.StreamFrame) {
	t.Helper()
	p := &streamPipe{recv: feed(fr)}
	data, cols, rows, err := p.Read()
	switch v := fr.GetFrame().(type) {
	case *gatewayv1.StreamFrame_Data:
		if err != nil || cols != 0 || rows != 0 || !bytes.Equal(data, v.Data) {
			t.Fatalf("Read(data %d bytes) = %d bytes, %dx%d, %v", len(v.Data), len(data), cols, rows, err)
		}
	case *gatewayv1.StreamFrame_Resize:
		checkResize(t, v.Resize, data, cols, rows, err)
	case *gatewayv1.StreamFrame_Close:
		wantClose(t, v.Close.GetReason(), data, cols, rows, err)
	default:
		wantClose(t, "end", data, cols, rows, err)
	}
}

// checkResize: a resize in the window is returned, any other is skipped.
func checkResize(t *testing.T, rz *gatewayv1.Resize, data []byte, cols, rows int, err error) {
	t.Helper()
	c, r := int(rz.GetCols()), int(rz.GetRows())
	if c < 1 || c > maxCols || r < 1 || r > maxRows {
		wantClose(t, "end", data, cols, rows, err)
		return
	}
	if err != nil || data != nil || cols != c || rows != r {
		t.Fatalf("Read(resize %dx%d) = %q, %dx%d, %v", c, r, data, cols, rows, err)
	}
}

// wantClose checks Read returned the close with reason: the frame's own, or
// the one after a skipped frame.
func wantClose(t *testing.T, reason string, data []byte, cols, rows int, err error) {
	t.Helper()
	var ce CloseError
	if !errors.As(err, &ce) || ce.Reason != reason || data != nil || cols != 0 || rows != 0 {
		t.Fatalf("Read = %q, %dx%d, %v; want close %q", data, cols, rows, err, reason)
	}
}

// checkCloseOnce: two Close calls send one close frame, then writes fail.
func checkCloseOnce(t *testing.T) {
	t.Helper()
	var sent []*gatewayv1.StreamResponse
	p := &streamPipe{send: func(r *gatewayv1.StreamResponse) error { sent = append(sent, r); return nil }}
	if err := p.Close("a"); err != nil {
		t.Fatalf("Close = %v", err)
	}
	if err := p.Close("b"); err != nil || len(sent) != 1 || sent[0].GetFrame().GetClose().GetReason() != "a" {
		t.Fatalf("second Close = %v, %d frames sent; want one close frame with reason a", err, len(sent))
	}
	if err := p.Write([]byte("x")); !errors.Is(err, errPipeClosed) || len(sent) != 1 {
		t.Fatalf("Write after Close = %v, %d frames sent", err, len(sent))
	}
}
